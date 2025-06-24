package headless

import (
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
	l := launcher.New().
		Bin(config.Get().ChroumiumBin).
		Headless(!config.Get().Headfull).
		Devtools(config.Get().DevTools)
	if config.Get().HeadlessUserDataDir != "" {
		l.UserDataDir(config.Get().HeadlessUserDataDir)
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
	Launcher.Cleanup()
	logger.Info("Headless launcher cleaned up")
}
