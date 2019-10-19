package wav

import (
	"errors"
	"fmt"
	"io"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pipelined/signal"
)

const (
	// value for wav output format chunk.
	wavOutFormat = 1
)

// ErrInvalidWav is returned when wav file is not valid.
var ErrInvalidWav = errors.New("invalid WAV")

type (
	// Pump reads wav data from ReadSeeker.
	Pump struct {
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

// Pump starts the pump process once executed, wav attributes are accessible.
func (p *Pump) Pump(sourceID string) (func(signal.Float64) error, signal.SampleRate, int, error) {
	p.decoder = wav.NewDecoder(p)
	if !p.decoder.IsValidFile() {
		return nil, 0, 0, ErrInvalidWav
	}

	numChannels := p.decoder.Format().NumChannels
	bitDepth := signal.BitDepth(p.decoder.BitDepth)

	// PCM buffer for wav decoder.
	PCMBuf := &audio.IntBuffer{
		Format:         p.decoder.Format(),
		SourceBitDepth: int(bitDepth),
	}

	// buffer for output conversion.
	ints := signal.InterInt{
		NumChannels: numChannels,
		BitDepth:    bitDepth,
	}
	if bitDepth == signal.BitDepth8 {
		ints.Unsigned = true
	}
	return func(b signal.Float64) error {
		// reset PCM buffer size.
		if ints.Size() != b.Size() {
			ints.Data = make([]int, b.Size()*numChannels)
			PCMBuf.Data = ints.Data
		}

		// read new buffer, io.EOF is never returned here.
		read, err := p.decoder.PCMBuffer(PCMBuf)
		if err != nil {
			return fmt.Errorf("error reading PCM buffer: %w", err)
		}
		if read == 0 {
			return io.EOF
		}

		// trim buffer.
		if read != len(ints.Data) {
			ints.Data = ints.Data[:read]
			for i := range b {
				b[i] = b[i][:ints.Size()]
			}
		}

		// copy to the output.
		ints.CopyToFloat64(b)
		return nil
	}, signal.SampleRate(p.decoder.SampleRate), numChannels, nil
}

// Flush flushes encoder.
func (s *Sink) Flush(string) error {
	if err := s.encoder.Close(); err != nil {
		return fmt.Errorf("error flushing WAV encoder: %w", err)
	}
	return nil
}

// Sink returns new Sink function instance.
func (s *Sink) Sink(pipeID string, sampleRate signal.SampleRate, numChannels int) (func(signal.Float64) error, error) {
	s.encoder = wav.NewEncoder(
		s,
		int(sampleRate),
		int(s.BitDepth),
		numChannels,
		wavOutFormat,
	)
	// buffer for input conversion.
	ints := signal.InterInt{
		BitDepth:    s.BitDepth,
		NumChannels: numChannels,
	}
	if s.BitDepth == signal.BitDepth8 {
		ints.Unsigned = true
	}
	// PCM buffer for write, refers data of ints buffer.
	PCMBuf := audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  int(sampleRate),
		},
		SourceBitDepth: int(s.BitDepth),
	}
	return func(b signal.Float64) error {
		if b.Size() != ints.Size() {
			ints.Data = make([]int, b.Size()*b.NumChannels())
		}
		b.CopyToInterInt(ints)
		PCMBuf.Data = ints.Data
		if err := s.encoder.Write(&PCMBuf); err != nil {
			return fmt.Errorf("error writing PCM buffer: %w", err)
		}
		return nil
	}, nil
}
