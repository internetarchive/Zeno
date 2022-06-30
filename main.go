/*
Copyright © 2020 Corentin Barreau <corentin.barreau24@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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

// Version - defined default version if it's not passed through flags during build
var Version string = "master"

func main() {
	app := cli.NewApp()
	app.Name = "Zeno"
	app.Version = utils.GetVersion()
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
