package inbox

import "errors"

var (
	ErrNotFound      = errors.New("inbox message not found")
	ErrNotOpen       = errors.New("inbox message is not open")
	ErrInvalidInput  = errors.New("invalid inbox input")
	ErrAlreadyClaimed = errors.New("inbox message already claimed or processed")
)
