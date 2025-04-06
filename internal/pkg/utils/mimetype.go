package utils

import "github.com/gabriel-vasile/mimetype"

// IsMIMETypeInHierarchy recursively checks if the MIME type and its parents match the expected MIME type
func IsMIMETypeInHierarchy(m *mimetype.MIME, expectedMIME string) bool {
	if m.Is(expectedMIME) {
		return true
	}

	parent := m.Parent()
	if parent == nil {
		return false
	}

	return IsMIMETypeInHierarchy(parent, expectedMIME)
}
