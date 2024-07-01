package version

import (
	"fmt"
	"runtime/debug"

	"github.com/urfave/cli/v2"
)

func newShowDepsCmd() *cli.Command {
	return &cli.Command{
		Name:   "deps",
		Usage:  "Get dependencies.",
		Action: cmdShowDeps,
	}
}

func cmdShowDeps(c *cli.Context) error {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			fmt.Printf("%s %s (%s)", dep.Path, dep.Version, dep.Sum)
			if dep.Replace != nil {
				fmt.Printf(" => %s %s (%s)", dep.Replace.Path, dep.Replace.Version, dep.Replace.Sum)
			} else {
				fmt.Print("\n")
			}
		}
	}

	return nil
}
