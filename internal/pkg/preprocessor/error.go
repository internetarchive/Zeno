package preprocessor

import "errors"

var (
	// ErrPreprocessorAlreadyInitialized is the error returned when the preprocessor is already initialized
	ErrPreprocessorAlreadyInitialized = errors.New("preprocessor already initialized")
	// ErrPreprocessorNotInitialized is the error returned when the preprocessor is not initialized
	ErrPreprocessorNotInitialized = errors.New("preprocessor not initialized")
	// ErrPreprocessorShuttingDown is the error returned when the preprocessor is shutting down
	ErrPreprocessorShuttingDown = errors.New("preprocessor shutting down")

	// ErrFeedbackItemNotPresent is the error returned when an item was sent to the feedback channel but not found in the state table
	ErrFeedbackItemNotPresent = errors.New("feedback item not present in state table")
	// ErrFinisehdItemNotFound is the error returned when an item been marked as finished but not found in the state table
	ErrFinisehdItemNotFound = errors.New("markAsFinished item not present in state table")
)
