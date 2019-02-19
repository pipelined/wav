package wav_test

import (
	"math"
	"testing"

	"github.com/pipelined/signal"
	"github.com/pipelined/wav"

	"github.com/stretchr/testify/assert"
)

const (
	bufferSize = 512
	wavSamples = 330534
	wav1       = "_testdata/sample.wav"
	wav2       = "_testdata/out1.wav"
	wav3       = "_testdata/out2.wav"
	wav8Bit    = "_testdata/sample8bit.wav"
	notWav     = "wav.go"
)

var (
	wavMessages = int(math.Ceil(float64(wavSamples) / float64(bufferSize)))
)

func TestWavPipe(t *testing.T) {
	tests := []struct {
		inFile  string
		outFile string
	}{
		{
			inFile:  wav1,
			outFile: wav2,
		},
		{
			inFile:  wav2,
			outFile: wav3,
		},
	}

	for _, test := range tests {
		pump := wav.NewPump(test.inFile)
		sink, err := wav.NewSink(test.outFile, signal.BitDepth16)
		assert.Nil(t, err)

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

		err = pump.Flush("")
		assert.Nil(t, err)
		err = sink.Flush("")
		assert.Nil(t, err)
	}
}

func TestWavPumpErrors(t *testing.T) {
	tests := []struct {
		path string
	}{
		{
			path: "non-existing file",
		},
		{
			path: notWav,
		},
		{
			path: wav8Bit,
		},
	}

	for _, test := range tests {
		pump := wav.NewPump(test.path)
		_, _, _, err := pump.Pump("", 0)
		assert.NotNil(t, err)
	}
}

func TestWavSinkErrors(t *testing.T) {
	// test unsupported bit depth
	_, err := wav.NewSink("test", signal.BitDepth8)
	assert.NotNil(t, err)

	// test empty file name
	sink, err := wav.NewSink("", signal.BitDepth16)
	assert.Nil(t, err)
	_, err = sink.Sink("test", 0, 0, 0)
	assert.NotNil(t, err)
}
