package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
  "runtime"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin"
)

type TemplaterPlugin struct {
	config gitspace_plugin.PluginConfig
}

var Plugin TemplaterPlugin

func (p *TemplaterPlugin) Name() string {
	if p.config.Metadata.Name != "" {
		return p.config.Metadata.Name
	}
	return "templater"
}

func (p *TemplaterPlugin) Version() string {
	if p.config.Metadata.Version != "" {
		return p.config.Metadata.Version
	}
	return "0.2.0"
}

func (p *TemplaterPlugin) Description() string {
	if p.config.Metadata.Description != "" {
		return p.config.Metadata.Description
	}
	return "Template manager for gitspace"
}

func (p TemplaterPlugin) Run(logger *log.Logger) error {
	logger.Info("Running templater plugin")
	return p.handleTemplatesMenu(logger)
}

func (p TemplaterPlugin) GetMenuOption() *huh.Option[string] {
	return &huh.Option[string]{
		Key:   "templates",
		Value: "Templates",
	}
}

func (p TemplaterPlugin) Standalone(args []string) error {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)
	logger.Info("Running templater plugin in standalone mode")
	return p.handleTemplatesMenu(logger)
}

func (p *TemplaterPlugin) SetConfig(config gitspace_plugin.PluginConfig) {
	p.config = config
}

func (p TemplaterPlugin) handleTemplatesMenu(logger *log.Logger) error {
	for {
		var choice string
		err := huh.NewSelect[string]().
			Title("Choose a templates action").
			Options(
				huh.NewOption("List templates", "list"),
				huh.NewOption("Create template", "create"),
				huh.NewOption("Apply template", "apply"),
				huh.NewOption("Go back", "back"),
			).
			Value(&choice).
			Run()

		if err != nil {
			return fmt.Errorf("error getting templates sub-choice: %w", err)
		}

		switch choice {
		case "list":
			p.listTemplates(logger)
		case "create":
			p.createTemplate(logger)
		case "apply":
			p.applyTemplate(logger)
		case "back":
			return nil
		default:
			logger.Error("Invalid templates sub-choice")
		}
	}
}

func (p TemplaterPlugin) listTemplates(logger *log.Logger) {
	logger.Info("Listing available templates...")
	// Implement logic to list templates
}

func (p TemplaterPlugin) createTemplate(logger *log.Logger) {
	logger.Info("Creating a new template...")
	// Implement logic to create a new template
}

func (p TemplaterPlugin) applyTemplate(logger *log.Logger) {
	logger.Info("Applying a template to a repository...")
	// Implement logic to apply a template
}

func main() {
	if err := Plugin.Standalone(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func (p *TemplaterPlugin) GetDependencies() map[string]string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil
	}
	pluginDir := filepath.Dir(filename)

	cmd := exec.Command("go", "list", "-m", "-json", "all")
	cmd.Dir = pluginDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var modules []struct {
		Path    string
		Version string
	}

	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var module struct {
			Path    string
			Version string
		}
		if err := decoder.Decode(&module); err != nil {
			return nil
		}
		modules = append(modules, module)
	}

	dependencies := make(map[string]string)
	for _, module := range modules {
		if module.Path != "github.com/ssotops/gitspace-catalog/plugins/templater" { // Exclude the main module
			dependencies[module.Path] = module.Version
		}
	}

	return dependencies
}
