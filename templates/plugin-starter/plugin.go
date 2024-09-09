package main

import (
	"fmt"
)

type PluginStarter struct{}

func (p *PluginStarter) Run() error {
	fmt.Println("Running plugin-starter")
	// TODO: Implement your plugin logic here
	return nil
}

func (p *PluginStarter) Name() string {
	return "plugin-starter"
}

func (p *PluginStarter) Version() string {
	return "0.1.0"
}

// This is the symbol that will be looked up by the plugin system
var Plugin PluginStarter
