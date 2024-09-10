package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
)

func updateCatalog(repoRoot string) error {
	catalogPath := filepath.Join(repoRoot, "gitspace-catalog.toml")

	catalog, err := loadCatalog(catalogPath)
	if err != nil {
		return fmt.Errorf("error loading catalog: %w", err)
	}

	preserveCatalogInfo(catalog)
	updatePlugins(catalog, repoRoot)
	updateTemplates(catalog, repoRoot)
	incrementVersion(catalog)
	updateLastUpdated(catalog)

	err = saveCatalog(catalog, catalogPath)
	if err != nil {
		return fmt.Errorf("error saving catalog: %w", err)
	}

	fmt.Println("Catalog updated successfully")
	return nil
}

func preserveCatalogInfo(catalog *toml.Tree) {
	if !catalog.Has("catalog") {
		catalog.Set("catalog", make(map[string]interface{}))
	}
	catalogInfo := catalog.Get("catalog").(*toml.Tree)
	if !catalogInfo.Has("name") {
		catalogInfo.Set("name", "Gitspace Official Catalog")
	}
	if !catalogInfo.Has("description") {
		catalogInfo.Set("description", "Official catalog of plugins and templates for Gitspace")
	}
	if !catalogInfo.Has("version") {
		catalogInfo.Set("version", "0.1.0")
	}
}

func loadCatalog(path string) (*toml.Tree, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return toml.Load(`[catalog]
name = "Gitspace Official Catalog"
description = "Official catalog of plugins and templates for Gitspace"
version = "0.1.0"

[plugins]

[templates]
`)
	}
	return toml.LoadFile(path)
}

func updatePlugins(catalog *toml.Tree, repoRoot string) {
	plugins := make(map[string]interface{})
	pluginsDir := filepath.Join(repoRoot, "plugins")

	filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if info.IsDir() && info.Name() != "plugins" {
			pluginInfo, err := loadPluginInfo(path)
			if err == nil {
				plugins[info.Name()] = pluginInfo
			}
		}
		return nil
	})

	catalog.Set("plugins", plugins)
}

func loadPluginInfo(pluginDir string) (map[string]interface{}, error) {
	tomlPath := filepath.Join(pluginDir, "gitspace-plugin.toml")
	tree, err := toml.LoadFile(tomlPath)
	if err != nil {
		return nil, err
	}

	info := make(map[string]interface{})
	info["version"] = tree.Get("plugin.version")
	info["description"] = tree.Get("plugin.description")
	info["path"] = pluginDir
	return info, nil
}

func updateTemplates(catalog *toml.Tree, repoRoot string) {
	templates := make(map[string]interface{})
	templatesDir := filepath.Join(repoRoot, "templates")

	filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if info.IsDir() && info.Name() != "templates" {
			templateInfo, err := loadTemplateInfo(path)
			if err == nil {
				templates[info.Name()] = templateInfo
			}
		}
		return nil
	})

	catalog.Set("templates", templates)
}

func loadTemplateInfo(templateDir string) (map[string]interface{}, error) {
	var tomlPath string
	if _, err := os.Stat(filepath.Join(templateDir, "gitspace-catalog.toml")); err == nil {
		tomlPath = filepath.Join(templateDir, "gitspace-catalog.toml")
	} else {
		tomlPath = filepath.Join(templateDir, "gitspace-plugin.toml")
	}

	tree, err := toml.LoadFile(tomlPath)
	if err != nil {
		return nil, err
	}

	info := make(map[string]interface{})
	if tree.Has("template") {
		info["version"] = tree.Get("template.version")
		info["description"] = tree.Get("template.description")
	} else if tree.Has("plugin") {
		info["version"] = tree.Get("plugin.version")
		info["description"] = tree.Get("plugin.description")
	}
	info["path"] = templateDir
	return info, nil
}

func incrementVersion(catalog *toml.Tree) {
	catalogInfo := catalog.Get("catalog").(*toml.Tree)
	version := catalogInfo.Get("version").(string)
	parts := strings.Split(version, ".")
	if len(parts) == 3 {
		patch := atoi(parts[2])
		newVersion := fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
		catalogInfo.Set("version", newVersion)
	}
}

func updateLastUpdated(catalog *toml.Tree) {
	lastUpdated := make(map[string]string)
	lastUpdated["date"] = time.Now().Format(time.RFC3339)
	lastUpdated["commit_hash"] = getLatestCommitHash()
	catalog.Set("catalog.last_updated", lastUpdated)
}

func getLatestCommitHash() string {
	// Implement logic to get the latest commit hash
	return "placeholder_hash"
}

func atoi(s string) int {
	i := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		i = i*10 + int(c-'0')
	}
	return i
}

func saveCatalog(catalog *toml.Tree, path string) error {
	return ioutil.WriteFile(path, []byte(catalog.String()), 0644)
}
