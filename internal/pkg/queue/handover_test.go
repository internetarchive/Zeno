package queue

import "testing"

func TestNewHandover(t *testing.T) {
	handover := NewHandoverChannel()
	if handover == nil {
		t.Fatal("expected handover channel to be created")
	}

	if handover.open.Load() {
		t.Fatal("expected handover channel to be closed")
	}
}

func TestHandoverTryOpen(t *testing.T) {
	handover := NewHandoverChannel()
	if !handover.TryOpen(1) {
		t.Fatal("expected handover channel to be opened")
	}

	if !handover.open.Load() {
		t.Fatal("expected handover channel to be open")
	}

	if handover.TryOpen(1) {
		t.Fatal("expected handover channel to not be opened again")
	}
}

func TestHandoverTryClose(t *testing.T) {
	handover := NewHandoverChannel()
	if handover.TryClose() {
		t.Fatal("expected handover channel to not be closed")
	}

	if !handover.TryOpen(1) {
		t.Fatal("expected handover channel to be opened")
	}

	if !handover.TryClose() {
		t.Fatal("expected handover channel to be closed")
	}

	if handover.TryClose() {
		t.Fatal("expected handover channel to not be closed again")
	}
}

func TestHandoverTryPut(t *testing.T) {
	handover := NewHandoverChannel()
	if handover.TryPut(&handoverEncodedItem{}) {
		t.Fatal("expected handover channel to not be open")
	}

	if !handover.TryOpen(1) {
		t.Fatal("expected handover channel to be opened")
	}

	if !handover.TryPut(&handoverEncodedItem{}) {
		t.Fatal("expected handover channel to accept item")
	}

	if handover.TryPut(&handoverEncodedItem{}) {
		t.Fatal("expected handover channel to not accept item")
	}
}

func TestHandoverTryGet(t *testing.T) {
	handover := NewHandoverChannel()
	if _, ok := handover.TryGet(); ok {
		t.Fatal("expected handover channel to not be open")
	}

	if !handover.TryOpen(1) {
		t.Fatal("expected handover channel to be opened")
	}

	if _, ok := handover.TryGet(); ok {
		t.Fatal("expected handover channel to not have any items")
	}

	if !handover.TryPut(&handoverEncodedItem{}) {
		t.Fatal("expected handover channel to accept item")
	}

	if _, ok := handover.TryGet(); !ok {
		t.Fatal("expected handover channel to have item")
	}

	if _, ok := handover.TryGet(); ok {
		t.Fatal("expected handover channel to not have any items")
	}
}
