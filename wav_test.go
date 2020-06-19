package wav_test

import (
	"context"
	"os"
	"testing"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
	"pipelined.dev/wav"
)

const (
	bufferSize = 512
	wavSample  = "_testdata/sample.wav"
	wav1       = "_testdata/out1.wav"
	wav2       = "_testdata/out2.wav"
	wav3       = "_testdata/out3.wav"
	notWav     = "wav.go"
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
		inFile, _ := os.Open(test.inPath)
		pump := wav.Source{ReadSeeker: inFile}

		outFile, _ := os.Create(test.outPath)
		sink := wav.Sink{
			WriteSeeker: outFile,
			BitDepth:    test.bitDepth,
		}
		line, err := pipe.Routing{
			Source: pump.Source(),
			Sink:   sink.Sink(),
		}.Line(bufferSize)

		p := pipe.New(context.Background(), pipe.WithLines(line))
		err = p.Wait()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}
