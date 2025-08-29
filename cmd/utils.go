package cmd

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/pyroscope-go"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
)

func startPyroscope() error {
	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	// Get the hostname via kernel
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("error getting hostname for Pyroscope: %w", err)
	}

	Version := utils.GetVersion()

	_, err = pyroscope.Start(pyroscope.Config{
		ApplicationName: "zeno",
		ServerAddress:   cfg.PyroscopeAddress,
		Logger:          nil,
		Tags:            map[string]string{"hostname": hostname, "job": cfg.Job, "version": Version.Version, "goVersion": Version.GoVersion, "uuid": uuid.New().String()[:5]},
		UploadRate:      15 * time.Second,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})

	if err != nil {
		panic(fmt.Errorf("error starting pyroscope: %w", err))
	}
	return nil
}
