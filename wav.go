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
var ErrInvalidWav = errors.New("Wav is not valid")

type (
	// Pump reads wav data from ReadSeeker.
	Pump struct {
		io.ReadSeeker
		d *wav.Decoder
	}

	// Sink writes wav data to WriteSeeker.
	// BitDepth is output bit depth. Supported values: 8, 16, 24 and 32.
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
func (p *Pump) Pump(sourceID string) (func(signal.Float64) error, signal.SampleRate, int, error) {
	decoder := wav.NewDecoder(p)
	if !decoder.IsValidFile() {
		return nil, 0, 0, ErrInvalidWav
	}

	p.d = decoder
	numChannels := decoder.Format().NumChannels
	bitDepth := signal.BitDepth(decoder.BitDepth)

	// PCM buffer for wav decoder.
	PCMBuf := &audio.IntBuffer{
		Format:         decoder.Format(),
		SourceBitDepth: int(bitDepth),
	}

	unsigned := false
	if bitDepth == signal.BitDepth8 {
		unsigned = true
	}
	// buffer for output conversion.
	ints := signal.InterInt{
		NumChannels: numChannels,
		BitDepth:    bitDepth,
		Unsigned:    unsigned,
	}
	var size int
	return func(b signal.Float64) error {
		// reset PCM buffer size.
		if b.Size() != size {
			size = b.Size()
			ints.Data = make([]int, size*numChannels)
			PCMBuf.Data = ints.Data
		}

		// read new buffer, io.EOF is never returned here.
		read, err := p.d.PCMBuffer(PCMBuf)
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
	}, signal.SampleRate(decoder.SampleRate), numChannels, nil
}

// Flush flushes encoder.
func (s *Sink) Flush(string) error {
	if err := s.e.Close(); err != nil {
		return fmt.Errorf("failed to flush wav encoder: %w", err)
	}
	return nil
}

// Sink returns new Sink function instance.
func (s *Sink) Sink(pipeID string, sampleRate signal.SampleRate, numChannels int) (func(signal.Float64) error, error) {
	s.e = wav.NewEncoder(s, int(sampleRate), int(s.BitDepth), numChannels, wavOutFormat)
	unsigned := false
	if s.BitDepth == signal.BitDepth8 {
		unsigned = true
	}
	// buffer for input conversion.
	ints := signal.InterInt{
		BitDepth:    s.BitDepth,
		NumChannels: numChannels,
		Unsigned:    unsigned,
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
		if err := s.e.Write(&PCMBuf); err != nil {
			return fmt.Errorf("failed to write PCM buffer: %w", err)
		}
		return nil
	}, nil
}
