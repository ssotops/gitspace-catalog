package main

import (
	"fmt"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
)

type GitspacePluginTemplates struct{}

func (p *GitspacePluginTemplates) GetMenuOption() huh.Option[string] {
	return huh.NewOption("Templates", "templates")
}

func (p *GitspacePluginTemplates) HandleMenuChoice(logger *log.Logger) error {
	return p.handleTemplatesMenu(logger)
}

func (p *GitspacePluginTemplates) Name() string {
	return "gitspace-plugin-templates"
}

func (p *GitspacePluginTemplates) Version() string {
	return "0.1.0"
}

func (p *GitspacePluginTemplates) handleTemplatesMenu(logger *log.Logger) error {
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

func (p *GitspacePluginTemplates) listTemplates(logger *log.Logger) {
	logger.Info("Listing available templates...")
	// Implement logic to list templates
}

func (p *GitspacePluginTemplates) createTemplate(logger *log.Logger) {
	logger.Info("Creating a new template...")
	// Implement logic to create a new template
}

func (p *GitspacePluginTemplates) applyTemplate(logger *log.Logger) {
	logger.Info("Applying a template to a repository...")
	// Implement logic to apply a template
}

// This is the symbol that will be looked up by the plugin system
var Plugin GitspacePluginTemplates
