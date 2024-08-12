package youtube

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	// Make io.ReadCloser from the youtube_test.html file
	f, err := os.Open("youtube_test.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Parse the video
	URLs, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}

	// Check the number of URLs
	expected := 146
	if len(URLs) != expected {
		t.Fatalf("Expected %d URLs, got %d", expected, len(URLs))
	}
}
