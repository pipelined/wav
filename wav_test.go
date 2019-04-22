package wav_test

import (
	"math"
	"os"
	"testing"

	"github.com/pipelined/signal"
	"github.com/pipelined/wav"

	"github.com/stretchr/testify/assert"
)

const (
	bufferSize = 512
	wavSamples = 330534
	wavSample  = "_testdata/sample.wav"
	wav1       = "_testdata/out1.wav"
	wav2       = "_testdata/out2.wav"
	wav3       = "_testdata/out3.wav"
	notWav     = "wav.go"
)

var (
	wavMessages = int(math.Ceil(float64(wavSamples) / float64(bufferSize)))
)

func TestWavPipe(t *testing.T) {
	tests := []struct {
		inPath   string
		outPath  string
		bitDepth signal.BitDepth
	}{
		{
			inPath:   wavSample,
			outPath:  wav1,
			bitDepth: signal.BitDepth16,
		},
		{
			inPath:   wav1,
			outPath:  wav2,
			bitDepth: signal.BitDepth8,
		},
		{
			inPath:   wav2,
			outPath:  wav3,
			bitDepth: signal.BitDepth24,
		},
	}

	for _, test := range tests {
		inFile, err := os.Open(test.inPath)
		assert.Nil(t, err)
		pump := wav.Pump{ReadSeeker: inFile}

		outFile, err := os.Create(test.outPath)
		assert.Nil(t, err)
		sink := wav.Sink{
			WriteSeeker: outFile,
			BitDepth:    test.bitDepth,
		}

		pumpFn, sampleRate, numChannles, err := pump.Pump("", bufferSize)
		assert.NotNil(t, pumpFn)
		assert.Nil(t, err)

		sinkFn, err := sink.Sink("", sampleRate, numChannles, bufferSize)
		assert.NotNil(t, sinkFn)
		assert.Nil(t, err)

		var buf [][]float64
		messages, samples := 0, 0
		for err == nil {
			buf, err = pumpFn()
			_ = sinkFn(buf)
			messages++
			if buf != nil {
				samples += len(buf[0])
			}
		}

		assert.Equal(t, wavMessages, messages)
		assert.Equal(t, wavSamples, samples)

		err = sink.Flush("")
		assert.Nil(t, err)

		err = inFile.Close()
		assert.Nil(t, err)
		err = outFile.Close()
		assert.Nil(t, err)
	}
}

func TestWavPumpErrors(t *testing.T) {
	f, _ := os.Open(notWav)
	pump := wav.Pump{ReadSeeker: f}
	_, _, _, err := pump.Pump("", 0)
	assert.NotNil(t, err)
}

func TestSupportedBitDepth(t *testing.T) {
	tests := map[signal.BitDepth]bool{
		signal.BitDepth8:    true,
		signal.BitDepth24:   true,
		signal.BitDepth(64): false,
	}

	for v, supported := range tests {
		err := wav.Supported.BitDepth(v)
		if supported {
			assert.Nil(t, err)
		} else {
			assert.NotNil(t, err)
		}
	}
}

func TestExtensions(t *testing.T) {
	exts := wav.Extensions()
	assert.Equal(t, 2, len(exts))
}
