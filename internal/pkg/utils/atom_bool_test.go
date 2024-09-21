package utils

import (
	"testing"
)

func TestAtomBool(t *testing.T) {
	atomBool := new(TAtomBool)
	atomBool.Set(true)

	if !atomBool.Get() {
		t.Errorf("Expected atomBool.Get() to be true, but got false")
	}
}
