package main

import (
	"fmt"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile("/tmp/nonexistent-config.yaml")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Printf("Error reading config: %v\n", err)
	} else {
		fmt.Printf("Config read successfully: %s\n", viper.ConfigFileUsed())
	}
}