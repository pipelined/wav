package wav

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
)

const (
	// value for wav output format chunk.
	wavOutFormat = 1
)

// ErrInvalidWav is returned when wav file is not valid.
var ErrInvalidWav = errors.New("invalid WAV")

type (
	// Source reads wav data from ReadSeeker.
	Source struct {
		io.ReadSeeker
	}

	// Sink writes wav data to WriteSeeker.
	// BitDepth is output bit depth. Supported values: 8, 16, 24 and 32.
	Sink struct {
		io.WriteSeeker
		signal.BitDepth
	}

	supported struct {
		bitDepths map[signal.BitDepth]struct{}
	}
)

// Source returns new wav source allocator closure.
func (s Source) Source() pipe.SourceAllocatorFunc {
	return func(bufferSize int) (pipe.Source, pipe.SignalProperties, error) {
		decoder := wav.NewDecoder(s)
		if !decoder.IsValidFile() {
			return pipe.Source{}, pipe.SignalProperties{}, ErrInvalidWav
		}

		channels := decoder.Format().NumChannels
		bitDepth := signal.BitDepth(decoder.BitDepth)

		// PCM buffer for wav decoder.
		pcm := audio.IntBuffer{
			Format:         decoder.Format(),
			SourceBitDepth: int(bitDepth),
			Data:           make([]int, bufferSize*channels),
		}
		alloc := signal.Allocator{
			Channels: channels,
			Capacity: bufferSize,
			Length:   bufferSize,
		}
		// 8-bits wav audio is encoded as unsigned signal
		var sourceFn pipe.SourceFunc
		if bitDepth == signal.BitDepth8 {
			sourceFn = sourceUnsigned(decoder, alloc.Uint8(bitDepth), pcm)
		} else {
			sourceFn = sourceSigned(decoder, alloc.Int64(bitDepth), pcm)
		}
		return pipe.Source{
				SourceFunc: sourceFn,
			},
			pipe.SignalProperties{
				SampleRate: signal.SampleRate(decoder.SampleRate),
				Channels:   channels,
			}, nil
	}
}

func sourceSigned(decoder *wav.Decoder, signed signal.Signed, pcm audio.IntBuffer) pipe.SourceFunc {
	return func(floating signal.Floating) (int, error) {
		// read new buffer, io.EOF is never returned here.
		read, err := decoder.PCMBuffer(&pcm)
		if err != nil {
			return 0, fmt.Errorf("error reading PCM buffer: %w", err)
		}
		if read == 0 {
			return 0, io.EOF
		}

		if read != len(pcm.Data) {
			read = signal.WriteInt(pcm.Data[:read], signed)
		} else {
			read = signal.WriteInt(pcm.Data, signed)
		}

		if read != floating.Length() {
			return signal.SignedAsFloating(signed.Slice(0, read), floating), nil
		}
		return signal.SignedAsFloating(signed, floating), nil
	}
}

func sourceUnsigned(decoder *wav.Decoder, unsigned signal.Unsigned, pcm audio.IntBuffer) pipe.SourceFunc {
	return func(floating signal.Floating) (int, error) {
		// read new buffer, io.EOF is never returned here.
		read, err := decoder.PCMBuffer(&pcm)
		if err != nil {
			return 0, fmt.Errorf("error reading PCM buffer: %w", err)
		}
		if read == 0 {
			return 0, io.EOF
		}

		for i := 0; i < read; i++ {
			unsigned.SetSample(i, uint64(pcm.Data[i]))
		}
		if read != len(pcm.Data) {
			return signal.UnsignedAsFloating(unsigned.Slice(0, signal.ChannelLength(read, unsigned.Channels())), floating), nil
		}
		return signal.UnsignedAsFloating(unsigned, floating), nil
	}
}

// Sink returns new wav sink allocator closure.
func (s Sink) Sink() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		encoder := wav.NewEncoder(
			s,
			int(props.SampleRate),
			int(s.BitDepth),
			props.Channels,
			wavOutFormat,
		)
		// PCM buffer for write, refers data of ints buffer.
		PCM := audio.IntBuffer{
			Format: &audio.Format{
				NumChannels: props.Channels,
				SampleRate:  int(props.SampleRate),
			},
			SourceBitDepth: int(s.BitDepth),
			Data:           make([]int, bufferSize*props.Channels),
		}

		alloc := signal.Allocator{
			Channels: props.Channels,
			Capacity: bufferSize,
			Length:   bufferSize,
		}
		// 8-bits wav audio is encoded as unsigned signal
		var sinkFn pipe.SinkFunc
		if s.BitDepth == signal.BitDepth8 {
			sinkFn = sinkUnsigned(encoder, alloc.Uint8(s.BitDepth), PCM)
		} else {
			sinkFn = sinkSigned(encoder, alloc.Int64(s.BitDepth), PCM)
		}
		return pipe.Sink{
			SinkFunc:  sinkFn,
			FlushFunc: encoderFlusher(encoder),
		}, nil
	}
}

func sinkSigned(encoder *wav.Encoder, ints signal.Signed, PCM audio.IntBuffer) pipe.SinkFunc {
	return func(floats signal.Floating) error {
		if n := signal.FloatingAsSigned(floats, ints); n != ints.Length() {
			PCM.Data = PCM.Data[:ints.Channels()*n]
			// defer because it must be done after write
			defer func() {
				PCM.Data = PCM.Data[:ints.Cap()]
			}()
		}
		signal.ReadInt(ints, PCM.Data)
		if err := encoder.Write(&PCM); err != nil {
			return fmt.Errorf("error writing PCM buffer: %w", err)
		}
		return nil
	}
}

func sinkUnsigned(encoder *wav.Encoder, uints signal.Unsigned, PCM audio.IntBuffer) pipe.SinkFunc {
	return func(floats signal.Floating) error {
		if n := signal.FloatingAsUnsigned(floats, uints); n != uints.Length() {
			PCM.Data = PCM.Data[:uints.Channels()*n]
			// defer because it must be done after write
			defer func() {
				PCM.Data = PCM.Data[:uints.Cap()]
			}()
		}
		for i := 0; i < len(PCM.Data); i++ {
			PCM.Data[i] = int(uints.Sample(i))
		}
		if err := encoder.Write(&PCM); err != nil {
			return fmt.Errorf("error writing PCM buffer: %w", err)
		}
		return nil
	}
}

func encoderFlusher(encoder *wav.Encoder) pipe.FlushFunc {
	return func(context.Context) error {
		if err := encoder.Close(); err != nil {
			return fmt.Errorf("error flushing WAV encoder: %w", err)
		}
		return nil
	}
}
