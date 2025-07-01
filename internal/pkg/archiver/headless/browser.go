package headless

import (
	"path"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var HeadlessBrowser *rod.Browser
var Launcher *launcher.Launcher

var logger = log.NewFieldedLogger(&log.Fields{"component": "archiver.headless.client"})

func Start() {
	var l *launcher.Launcher
	if config.Get().HeadlessUserMode {
		// In user mode, we use the default launcher
		l = launcher.NewUserMode()
	} else {
		l = launcher.New()
	}
	l.Bin(config.Get().HeadlessChroumiumBin).
		Headless(!config.Get().HeadlessHeadfull).
		Devtools(config.Get().HeadlessDevTools)
	if config.Get().HeadlessUserDataDir != "" {
		l.UserDataDir(config.Get().HeadlessUserDataDir)
	} else {
		l.UserDataDir(path.Join(config.Get().WARCTempDir, "headless-user-data"))
	}
	controlerURL := l.MustLaunch()
	HeadlessBrowser = rod.New().
		ControlURL(controlerURL).
		DefaultDevice(devices.Clear).
		Trace(config.Get().HeadlessTrace).
		MustConnect()
	Launcher = l
}

func Close() {
	HeadlessBrowser.Close()
	logger.Info("Headless browser closed")
	if config.Get().HeadlessUserMode {
		// In user mode, we DONT clean up the launcher to preserve user-data
		logger.Info("Headless browser in user mode, not cleaning up launcher")
		return
	}
	Launcher.Cleanup()
	logger.Info("Headless launcher cleaned up")
}
