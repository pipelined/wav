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
	// DefaultExtension of wav files.
	DefaultExtension = ".wav"
)

var (
	// ErrInvalidWav is returned when wav file is not valid.
	ErrInvalidWav = errors.New("Wav is not valid")

	// Supported is the struct that provides validation logic for wav package values.
	Supported = supported{
		bitDepths: map[signal.BitDepth]struct{}{
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

	supported struct {
		bitDepths map[signal.BitDepth]struct{}
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

// Extensions of wav audio files.
func Extensions() []string {
	return extensions
}

// BitDepth checks if provided bit depth is supported.
func (s supported) BitDepth(v signal.BitDepth) error {
	if _, ok := s.bitDepths[v]; !ok {
		return fmt.Errorf("Bit depth %v is not supported", v)
	}
	return nil
}

// BitDepths returns a map of supported bit depths.
func (s supported) BitDepths() map[signal.BitDepth]struct{} {
	result := make(map[signal.BitDepth]struct{})
	for k, v := range s.bitDepths {
		result[k] = v
	}
	return result
}
