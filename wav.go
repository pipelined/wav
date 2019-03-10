package wav

import (
	"errors"
	"io"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/pipelined/signal"
)

// value for wav output format chunk
const wavOutFormat = 1

var (
	// ErrInvalidWav is returned when wav file is not valid.
	ErrInvalidWav = errors.New("Wav is not valid")
)

type (
	// Pump reads from wav file.
	// This component cannot be reused for consequent runs.
	Pump struct {
		r io.ReadSeeker
		d *wav.Decoder
	}

	// Sink sink saves audio to wav file.
	Sink struct {
		w        io.WriteSeeker
		e        *wav.Encoder
		bitDepth signal.BitDepth
	}
)

// NewPump creates a new wav pump and sets wav props.
func NewPump(r io.ReadSeeker) *Pump {
	return &Pump{r: r}
}

// Pump starts the pump process once executed, wav attributes are accessible.
func (p *Pump) Pump(sourceID string, bufferSize int) (func() ([][]float64, error), int, int, error) {
	decoder := wav.NewDecoder(p.r)
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

// NewSink creates new wav sink.
func NewSink(w io.WriteSeeker, bitDepth signal.BitDepth) *Sink {
	return &Sink{
		w:        w,
		bitDepth: bitDepth,
	}
}

// Flush flushes encoder.
func (s *Sink) Flush(string) error {
	return s.e.Close()
}

// Sink returns new Sink function instance.
func (s *Sink) Sink(pipeID string, sampleRate, numChannels, bufferSize int) (func([][]float64) error, error) {
	s.e = wav.NewEncoder(s.w, sampleRate, int(s.bitDepth), numChannels, wavOutFormat)
	ib := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: numChannels,
			SampleRate:  sampleRate,
		},
		SourceBitDepth: int(s.bitDepth),
	}

	unsigned := false
	if s.bitDepth == signal.BitDepth8 {
		unsigned = true
	}

	return func(b [][]float64) error {
		ib.Data = signal.Float64(b).AsInterInt(s.bitDepth, unsigned)
		return s.e.Write(ib)
	}, nil
}
