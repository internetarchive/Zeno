package config

import (
	"testing"
)

func TestInitConfig_Defaults(t *testing.T) {
	err := InitConfig()
	if err != nil {
		t.Fatalf("Cannot init config %v", err)
	}
	config := Get()

	// HQBatchSize is set to 100 by default in InitConfig.
	if config.HQBatchSize != 100 {
		t.Fatalf("HQBatchSize default isn't set to 100 but %d", config.HQBatchSize)
	}
}
