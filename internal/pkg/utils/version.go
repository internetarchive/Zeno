package utils

import (
	"runtime/debug"
)

type Version struct {
	Version     string
	GoVersion   string
	WarcVersion string
	ZenoVersion string
}

func GetVersion() (version Version) {
	// Defaults to "unknown_version"
	version.Version = "unknown_version"

	if info, ok := debug.ReadBuildInfo(); ok {
		// Determine Zeno's version based on Git data
		for _, setting := range info.Settings {
			// This returns the current git hash
			if setting.Key == "vcs.revision" {
				version.Version = setting.Value
			}

			// This would show us if the current git tree is modified from the hash, possible changes that weren't committed
			if setting.Key == "vcs.modified" {
				if setting.Value == "true" {
					version.Version += " (modified)"
				}
			}
		}

		for _, dep := range info.Deps {
			if dep.Path == "github.com/CorentinB/warc" {
				version.WarcVersion = dep.Version
			}
		}

		// Get the Go version used to build Zeno
		version.GoVersion = info.GoVersion
	}

	return version
}
