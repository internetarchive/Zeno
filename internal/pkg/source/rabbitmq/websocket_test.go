package hq

import (
	"testing"
)

func TestHandleConfirmedMsg(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name: "valid JSON",
			input: []byte(`{
				"type": "confirm",
				"payload": {
					"project": "demo",
					"goVersion": "1.22"
				}
			}`),
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   []byte(`{ "type": "confirm", `), // broken JSON
			wantErr: true,
		},
		{
			name: "extra fields JSON",
			input: []byte(`{
				"type": "confirm",
				"payload": {
					"project": "demo",
					"goVersion": "1.22",
					"extra": "ignored"
				},
				"unknown": "field"
			}`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleConfirmedMsg(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleConfirmedMsg() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
