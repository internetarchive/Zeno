package config

type Flags struct {
	Workers  int
	MaxHops  uint
	Headless bool
	JSON     bool
	Debug    bool
}

type Application struct {
	Flags Flags
}

var App *Application

func init() {
	App = &Application{}
}
