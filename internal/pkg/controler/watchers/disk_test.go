package watchers

import (
	"testing"
)

func TestCheckDiskUsage(t *testing.T) {
	tests := []struct {
		name      string
		total     uint64
		free      uint64
		wantError bool
	}{
		{
			name:      "Low disk space on large disk",
			total:     300 * 1024 * 1024 * 1024, // 300 GiB
			free:      15 * 1024 * 1024 * 1024,  // 15 GiB
			wantError: true,
		},
		{
			name:      "Sufficient disk space on large disk",
			total:     300 * 1024 * 1024 * 1024, // 300 GiB
			free:      50 * 1024 * 1024 * 1024,  // 50 GiB
			wantError: false,
		},
		{
			name:      "Low disk space on small disk",
			total:     100 * 1024 * 1024 * 1024, // 100 GiB
			free:      3 * 1024 * 1024 * 1024,   // 3 GiB
			wantError: true,
		},
		{
			name:      "Sufficient disk space on small disk",
			total:     100 * 1024 * 1024 * 1024, // 100 GiB
			free:      60 * 1024 * 1024 * 1024,  // 10 GiB
			wantError: false,
		},
		{
			name:      "Edge case: exactly at threshold for small disk",
			total:     300 * 1024 * 1024 * 1024,                                                                        // 200 GiB
			free:      uint64((50 * 1024 * 1024 * 1024) * (float64(300*1024*1024*1024) / float64(256*1024*1024*1024))), // Threshold value
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDiskUsage(tt.total, tt.free)
			if (err != nil) != tt.wantError {
				t.Errorf("checkDiskUsage() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
