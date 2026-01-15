package core

import "errors"

var (
	ErrBadArguments    = errors.New("arguments are not acceptable")
	ErrNilDependency   = errors.New("search service: nil dependency")
	ErrRequestTooLarge = errors.New("request is too large")
)
