package output

import "fmt"

const (
	ExitOK         = 0
	ExitAPI        = 1
	ExitValidation = 2
	ExitAuth       = 3
	ExitNetwork    = 4
	ExitInternal   = 5
)

type ErrDetail struct {
	Type       string      `json:"type"`
	Code       int         `json:"code,omitempty"`
	Message    string      `json:"message"`
	Hint       string      `json:"hint,omitempty"`
	ConsoleURL string      `json:"console_url,omitempty"`
	Detail     interface{} `json:"detail,omitempty"`
}

type ExitError struct {
	Code   int
	Detail *ErrDetail
	Err    error
	Raw    bool
}

func (e *ExitError) Error() string {
	if e.Detail != nil {
		return e.Detail.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func ErrWithHint(code int, errType, msg, hint string) *ExitError {
	return &ExitError{
		Code:   code,
		Detail: &ErrDetail{Type: errType, Message: msg, Hint: hint},
	}
}
