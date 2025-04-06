package stats

import (
	"testing"
)

func TestMean_Add(t *testing.T) {
	m := &mean{}

	m.add(10)
	if got := m.get(); got != 10 {
		t.Errorf("add(10) = %v, want %v", got, 10)
	}

	m.add(20)
	if got := m.get(); got != 15 {
		t.Errorf("add(20) = %v, want %v", got, 15)
	}
}

func TestMean_Get(t *testing.T) {
	m := &mean{}

	if got := m.get(); got != 0 {
		t.Errorf("get() = %v, want %v", got, 0)
	}

	m.add(10)
	m.add(20)
	if got := m.get(); got != 15 {
		t.Errorf("get() = %v, want %v", got, 15)
	}
}

func TestMean_Reset(t *testing.T) {
	m := &mean{}

	m.add(10)
	m.add(20)
	m.reset()

	if got := m.get(); got != 0 {
		t.Errorf("reset() = %v, want %v", got, 0)
	}
}
