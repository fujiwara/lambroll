package lambroll_test

import (
	"errors"
	"testing"

	"github.com/fujiwara/lambroll"
)

var errGeneral = errors.New("error")

var ExitErrorTests = []struct {
	err            error
	code           int
	extractedError error
}{
	{lambroll.ErrDiff, 2, nil},
	{errGeneral, 1, errGeneral},
	{nil, 0, nil},
}

func TestExitError(t *testing.T) {
	for _, tt := range ExitErrorTests {
		code, err := lambroll.ExtractExitCodeAndError(tt.err)
		if code != tt.code {
			t.Errorf("ExtractExitCode(%v) => %d, want %d", tt.err, code, tt.code)
		}
		if err != nil && tt.extractedError != nil && err != tt.extractedError {
			t.Errorf("ExtractExitCode(%v) => %v, want %v", tt.err, err, tt.extractedError)
		}
	}
}
