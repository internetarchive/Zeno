//go:build !windows

package watchers

import (
	"fmt"
	"syscall"

	"github.com/internetarchive/Zeno/internal/pkg/config"
)

func CheckDiskUsage(path string) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		panic(fmt.Sprintf("Error retrieving disk stats: %v\n", err))
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)

	return checkThreshold(total, free, config.Get().MinSpaceRequired)
}
