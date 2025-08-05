package headless

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
)

var HeadlessBrowser *rod.Browser
var Launcher *launcher.Launcher

const MagicLatestChromiumRevision = -1

var browserLogger = log.NewFieldedLogger(&log.Fields{"component": "archiver.headless.client"})

func queryLatestChromiumRevision() (int, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	// From web page: https://chromiumdash.appspot.com/releases?platform=Linux
	resp, err := client.Get("https://chromiumdash.appspot.com/fetch_releases?channel=Stable&platform=Linux&num=1&offset=0")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get latest Chromium revision: %s", resp.Status)
	}

	var data []struct {
		Revision int `json:"chromium_main_branch_position"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, fmt.Errorf("no revisions found")
	}

	return data[0].Revision, nil
}

func Start() {
	var l *launcher.Launcher
	if config.Get().HeadlessUserMode {
		// In user mode, we use the default launcher
		l = launcher.NewUserMode()
	} else {
		l = launcher.New()
	}

	if config.Get().HeadlessChromiumRevision == MagicLatestChromiumRevision {
		latestRev, err := queryLatestChromiumRevision()
		if err != nil {
			browserLogger.Error("failed to query latest Chromium revision, you can try to specify the revision manually", "err", err)
			os.Exit(1)
		}
		browserLogger.Info("using latest Chromium revision", "revision", latestRev)
		config.Get().HeadlessChromiumRevision = latestRev
	}

	l.Bin(config.Get().HeadlessChromiumBin).
		Revision(config.Get().HeadlessChromiumRevision).
		Headless(!config.Get().HeadlessHeadful).
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

	if HeadlessBrowser.MustVersion().ProtocolVersion != "1.3" {
		panic(fmt.Sprintf("Unsupported DevTools-Protocol version: %s, expected 1.3", HeadlessBrowser.MustVersion().ProtocolVersion))
	}
}

func Close() {
	HeadlessBrowser.Close()
	browserLogger.Info("Headless browser closed")
	if config.Get().HeadlessUserMode {
		// In user mode, we DONT clean up the launcher to preserve user-data
		browserLogger.Info("Headless browser in user mode, not cleaning up launcher")
		return
	}
	Launcher.Cleanup()
	browserLogger.Info("Headless launcher cleaned up")
}
