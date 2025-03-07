package lq

import "errors"

var (
	//  is the error returned when the postprocessor is already initialized
	ErrLQAlreadyInitialized = errors.New("lq client already initialized")
)
