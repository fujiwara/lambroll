package lambroll

import "errors"

type ExitCode int

const (
	ExitCodeOK           = ExitCode(0)
	ExitCodeGeneralError = ExitCode(1)
	ExitCodeDiffFound    = ExitCode(2)
)

var ErrDiff = &ExitError{Code: ExitCodeDiffFound, Err: nil}

type ExitError struct {
	Code ExitCode
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return nil
}

// ExtractExitCode extracts exit code from error.
func extractExitCodeAndError(err error) (int, error) {
	if err == nil {
		return int(ExitCodeOK), nil
	}
	var e *ExitError
	if errors.As(err, &e) {
		return int(e.Code), e.Err
	}
	return int(ExitCodeGeneralError), err
}
