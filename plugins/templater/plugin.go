package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace/gsplugin"
)

type TemplaterPlugin struct {
	config gsplugin.PluginConfig
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

func (p *TemplaterPlugin) SetConfig(config gsplugin.PluginConfig) {
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
