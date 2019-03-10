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
	wav1       = "_testdata/sample.wav"
	wav2       = "_testdata/out1.wav"
	wav3       = "_testdata/out2.wav"
	wav4       = "_testdata/out3.wav"
	wav8Bit    = "_testdata/sample8bit.wav"
	notWav     = "wav.go"
)

var (
	wavMessages = int(math.Ceil(float64(wavSamples) / float64(bufferSize)))
)

func TestWavPipe(t *testing.T) {
	tests := []struct {
		inPath  string
		outPath string
	}{
		{
			inPath:  wav1,
			outPath: wav2,
		},
		{
			inPath:  wav2,
			outPath: wav3,
		},
		{
			inPath:  wav8Bit,
			outPath: wav4,
		},
	}

	for _, test := range tests {
		inFile, err := os.Open(test.inPath)
		assert.Nil(t, err)
		pump := wav.NewPump(inFile)

		outFile, err := os.Create(test.outPath)
		assert.Nil(t, err)
		sink := wav.NewSink(outFile, signal.BitDepth16)

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
	pump := wav.NewPump(f)
	_, _, _, err := pump.Pump("", 0)
	assert.NotNil(t, err)
}
