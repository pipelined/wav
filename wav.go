package wav

import (
	"errors"
	"fmt"
	"io"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pipelined/pipe"
	"github.com/pipelined/signal"
)

const (
	// value for wav output format chunk.
	wavOutFormat = 1
	// DefaultExtension of wav files.
	DefaultExtension = ".wav"
)

var (
	// ErrInvalidWav is returned when wav file is not valid.
	ErrInvalidWav = errors.New("Wav is not valid")

	// Supported is the struct with supported parameters of wav output.
	Supported = struct {
		BitDepths map[signal.BitDepth]struct{}
	}{
		BitDepths: map[signal.BitDepth]struct{}{
			signal.BitDepth8:  {},
			signal.BitDepth16: {},
			signal.BitDepth24: {},
			signal.BitDepth32: {},
		},
	}
	// extensions of wav files.
	extensions = []string{DefaultExtension, ".wave"}
)

type (
	// Pump reads from wav file.
	// ReadSeeker is the source of wav data.
	Pump struct {
		io.ReadSeeker
		d *wav.Decoder
	}

	// Sink sink saves audio to wav file.
	// WriteSeeker is the destination of wav data.
	// BitDepth is output bit depth.
	Sink struct {
		io.WriteSeeker
		signal.BitDepth
		e *wav.Encoder
	}

	// SinkBuilder creates Sink with provided parameters.
	SinkBuilder struct {
		io.WriteSeeker
		signal.BitDepth
	}
)

// Pump starts the pump process once executed, wav attributes are accessible.
func (p *Pump) Pump(sourceID string, bufferSize int) (func() ([][]float64, error), int, int, error) {
	decoder := wav.NewDecoder(p)
	if !decoder.IsValidFile() {
		return nil, 0, 0, ErrInvalidWav
	}

	p.d = decoder
	numChannels := decoder.Format().NumChannels
	sampleRate := int(decoder.SampleRate)
	bitDepth := signal.BitDepth(decoder.BitDepth)

	ib := &audio.IntBuffer{
		Format:         decoder.Format(),
		Data:           make([]int, bufferSize*numChannels),
		SourceBitDepth: int(bitDepth),
	}

	unsigned := false
	if bitDepth == signal.BitDepth8 {
		unsigned = true
	}

	return func() ([][]float64, error) {
		readSamples, err := p.d.PCMBuffer(ib)
		if err != nil {
			return nil, err
		}

		if readSamples == 0 {
			return nil, io.EOF
		}

		// prune buffer to actual size
		b := signal.InterInt{
			Data:        ib.Data[:readSamples],
			NumChannels: numChannels,
			BitDepth:    bitDepth,
			Unsigned:    unsigned,
		}.AsFloat64()

		if b.Size() != bufferSize {
			return b, io.ErrUnexpectedEOF
		}
		return b, nil
	}, sampleRate, numChannels, nil
}

// Flush flushes encoder.
func (s *Sink) Flush(string) error {
	return s.e.Close()
}

// Sink returns new Sink function instance.
func (s *Sink) Sink(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error) {
	s.e = wav.NewEncoder(s, sampleRate, int(s.BitDepth), numChannels, wavOutFormat)
	ib := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: int(s.BitDepth),
	}

	unsigned := false
	if s.BitDepth == signal.BitDepth8 {
		unsigned = true
	}

	return func(b [][]float64) error {
		ib.Data = signal.Float64(b).AsInterInt(s.BitDepth, unsigned)
		return s.e.Write(ib)
	}, nil
}

// Build creates wav sink if configuration is valid, otherwise error is returned.
func (sb *SinkBuilder) Build() (pipe.Sink, error) {
	// check if bit depth is supported
	if _, ok := Supported.BitDepths[sb.BitDepth]; !ok {
		return nil, fmt.Errorf("Bit depth %v is not supported", sb.BitDepth)
	}

	return &Sink{
		WriteSeeker: sb.WriteSeeker,
		BitDepth:    sb.BitDepth,
	}, nil
}

// Extensions of wav audio files.
func Extensions() []string {
	return extensions
}
