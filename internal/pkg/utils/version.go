package utils

import "runtime/debug"

type Version struct {
	Version   string
	GoVersion string
}

func GetVersion() (version Version) {
	// Defaults to master
	version.Version = "master"

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

		// Get the Go version used to build Zeno
		version.GoVersion = info.GoVersion
	}

	return version
}
