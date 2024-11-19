package preprocessor

import "errors"

var (
	// ErrPreprocessorAlreadyInitialized is the error returned when the preprocessor is already initialized
	ErrPreprocessorAlreadyInitialized = errors.New("preprocessor already initialized")
)
