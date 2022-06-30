package main

import (
	"os"

	_ "net/http/pprof"

	"github.com/CorentinB/Zeno/cmd"
	_ "github.com/CorentinB/Zeno/cmd/all"
	"github.com/CorentinB/Zeno/internal/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "Zeno"
	app.Version = utils.GetVersion().Version
	app.Authors = append(app.Authors, &cli.Author{Name: "Corentin Barreau", Email: "corentin@archive.org"})
	app.Usage = ""

	app.Flags = cmd.GlobalFlags
	app.Commands = cmd.Commands
	app.CommandNotFound = cmd.CommandNotFound
	app.Before = func(context *cli.Context) error {
		return nil
	}

	app.After = func(context *cli.Context) error {
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Panic(err)
	}
}
