package index

import "errors"

var (
	// ErrHostNotFound is returned when the host is not found in the index
	ErrHostNotFound = errors.New("host not found")

	// ErrHostCantBeEmpty is returned when the given host is empty
	ErrHostCantBeEmpty = errors.New("host cannot be empty")

	// ErrHostEmpty is returned when the host is empty
	ErrHostEmpty = errors.New("host is empty")
)
