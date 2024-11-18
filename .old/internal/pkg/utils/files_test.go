package utils

import (
	"os"
	"testing"
)

func TestFileExists(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "fileexists_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file
	filePath := tempDir + "/a"
	err = os.WriteFile(filePath, []byte("file"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Test existing file
	if !FileExists(filePath) {
		t.Errorf("Expected file %s to exist, but it doesn't", filePath)
	}

	// Test non-existing file
	nonExistentPath := tempDir + "/b"
	if FileExists(nonExistentPath) {
		t.Errorf("Expected file %s to not exist, but it does", nonExistentPath)
	}
}
