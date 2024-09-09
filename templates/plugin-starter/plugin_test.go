package main

import (
	"testing"
)

func TestPluginStarter(t *testing.T) {
	plugin := &PluginStarter{}

	// Test Name method
	if plugin.Name() != "plugin-starter" {
		t.Errorf("Expected plugin name to be 'plugin-starter', got '%s'", plugin.Name())
	}

	// Test Version method
	if plugin.Version() != "0.1.0" {
		t.Errorf("Expected plugin version to be '0.1.0', got '%s'", plugin.Version())
	}

	// Test Run method
	err := plugin.Run()
	if err != nil {
		t.Errorf("Plugin Run() method returned an error: %v", err)
	}

	// TODO: Add more specific tests for your plugin's functionality
}
