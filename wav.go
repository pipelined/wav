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
		decoder *wav.Decoder
	}

	// Sink writes wav data to WriteSeeker.
	// BitDepth is output bit depth. Supported values: 8, 16, 24 and 32.
	Sink struct {
		io.WriteSeeker
		signal.BitDepth
		encoder *wav.Encoder
	}

	supported struct {
		bitDepths map[signal.BitDepth]struct{}
	}
)

// Source returns new wav source allocator closure.
func (p *Source) Source() pipe.SourceAllocatorFunc {
	return func(bufferSize int) (pipe.Source, pipe.SignalProperties, error) {
		p.decoder = wav.NewDecoder(p)
		if !p.decoder.IsValidFile() {
			return pipe.Source{}, pipe.SignalProperties{}, ErrInvalidWav
		}

		channels := p.decoder.Format().NumChannels
		bitDepth := signal.BitDepth(p.decoder.BitDepth)

		// PCM buffer for wav decoder.
		PCM := audio.IntBuffer{
			Format:         p.decoder.Format(),
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
			sourceFn = p.sourceUnsigned(alloc.Uint8(bitDepth), PCM)
		} else {
			sourceFn = p.sourceSigned(alloc.Int64(bitDepth), PCM)
		}
		return pipe.Source{
				SourceFunc: sourceFn,
			},
			pipe.SignalProperties{
				SampleRate: signal.SampleRate(p.decoder.SampleRate),
				Channels:   channels,
			}, nil
	}
}

func (p *Source) sourceSigned(signed signal.Signed, PCM audio.IntBuffer) pipe.SourceFunc {
	return func(floating signal.Floating) (int, error) {
		// read new buffer, io.EOF is never returned here.
		read, err := p.decoder.PCMBuffer(&PCM)
		if err != nil {
			return 0, fmt.Errorf("error reading PCM buffer: %w", err)
		}
		if read == 0 {
			return 0, io.EOF
		}

		if read != len(PCM.Data) {
			read = signal.WriteInt(PCM.Data[:read], signed)
		} else {
			read = signal.WriteInt(PCM.Data, signed)
		}

		if read != floating.Length() {
			return signal.SignedAsFloating(signed.Slice(0, read), floating), nil
		}
		return signal.SignedAsFloating(signed, floating), nil
	}
}

func (p *Source) sourceUnsigned(unsigned signal.Unsigned, PCM audio.IntBuffer) pipe.SourceFunc {
	return func(floating signal.Floating) (int, error) {
		// read new buffer, io.EOF is never returned here.
		read, err := p.decoder.PCMBuffer(&PCM)
		if err != nil {
			return 0, fmt.Errorf("error reading PCM buffer: %w", err)
		}
		if read == 0 {
			return 0, io.EOF
		}

		for i := 0; i < read; i++ {
			unsigned.SetSample(i, uint64(PCM.Data[i]))
		}
		if read != len(PCM.Data) {
			return signal.UnsignedAsFloating(unsigned.Slice(0, signal.ChannelLength(read, unsigned.Channels())), floating), nil
		}
		return signal.UnsignedAsFloating(unsigned, floating), nil
	}
}

// Flush flushes encoder.
func (s *Sink) Flush(context.Context) error {
	if err := s.encoder.Close(); err != nil {
		return fmt.Errorf("error flushing WAV encoder: %w", err)
	}
	return nil
}

// Sink returns new wav sink allocator closure.
func (s *Sink) Sink() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		s.encoder = wav.NewEncoder(
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
			sinkFn = s.sinkUnsigned(alloc.Uint8(s.BitDepth), PCM)
		} else {
			sinkFn = s.sinkSigned(alloc.Int64(s.BitDepth), PCM)
		}
		return pipe.Sink{
			SinkFunc:  sinkFn,
			FlushFunc: s.Flush,
		}, nil
	}
}

func (s *Sink) sinkSigned(ints signal.Signed, PCM audio.IntBuffer) pipe.SinkFunc {
	return func(floats signal.Floating) error {
		if n := signal.FloatingAsSigned(floats, ints); n != ints.Length() {
			PCM.Data = PCM.Data[:ints.Channels()*n]
			// defer because it must be done after write
			defer func() {
				PCM.Data = PCM.Data[:ints.Cap()]
			}()
		}
		signal.ReadInt(ints, PCM.Data)
		if err := s.encoder.Write(&PCM); err != nil {
			return fmt.Errorf("error writing PCM buffer: %w", err)
		}
		return nil
	}
}

func (s *Sink) sinkUnsigned(uints signal.Unsigned, PCM audio.IntBuffer) pipe.SinkFunc {
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
		if err := s.encoder.Write(&PCM); err != nil {
			return fmt.Errorf("error writing PCM buffer: %w", err)
		}
		return nil
	}
}
