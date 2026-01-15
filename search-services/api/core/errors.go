package core

import "errors"

var ErrBadArguments = errors.New("arguments are not acceptable")
var ErrAlreadyExists = errors.New("resource or task already exists")
var ErrNotFound = errors.New("resource is not found")
var ErrBadLimit = errors.New("limit must be positive")
var ErrBadPhrase = errors.New("phrase must be non-empty")
