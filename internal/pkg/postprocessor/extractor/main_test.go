package extractor

import (
	"os"
	"testing"

	"go.uber.org/goleak"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

func TestMain(m *testing.M) {
	config.InitConfig()
	goleak.VerifyTestMain(m)
	os.Exit(m.Run())
}
