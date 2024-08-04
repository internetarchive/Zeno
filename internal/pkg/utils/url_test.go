package utils

import (
	"net/url"
	"testing"
)

func TestURLToStringPunycode(t *testing.T) {
	u, err := url.Parse("https://xn----8sbddjhbicfsohgbg1aeo.xn--p1ia/pic/file/map_of_sarlat.pdf")
	if err != nil {
		t.Fatalf("Error parsing URL: %v", err)
	}

	expected := "https://xn----8sbddjhbicfsohgbg1aeo.xn--p1ia/pic/file/map_of_sarlat.pdf"
	actual := URLToString(u)
	if actual != expected {
		t.Fatalf("Expected %s, got %s", expected, actual)
	}
}

func TestURLToStringPunycodeWithPort(t *testing.T) {
	u, err := url.Parse("https://xn----8sbddjhbicfsohgbg1aeo.xn--p1ia:8080/pic/file/map_of_sarlat.pdf")
	if err != nil {
		t.Fatalf("Error parsing URL: %v", err)
	}

	expected := "https://xn----8sbddjhbicfsohgbg1aeo.xn--p1ia:8080/pic/file/map_of_sarlat.pdf"
	actual := URLToString(u)
	if actual != expected {
		t.Fatalf("Expected %s, got %s", expected, actual)
	}
}
