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

	// Check if the file exists
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		return fmt.Errorf("gitspace-catalog.toml not found at %s", catalogPath)
	}

	catalog, err := loadCatalog(catalogPath)
	if err != nil {
		return fmt.Errorf("error loading catalog: %w", err)
	}

	// Preserve existing catalog information
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
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return toml.LoadBytes(data)
}

func updatePlugins(catalog *toml.Tree, repoRoot string) {
	plugins := make(map[string]interface{})
	if pluginsTree := catalog.Get("plugins"); pluginsTree != nil {
		if tree, ok := pluginsTree.(*toml.Tree); ok {
			for _, key := range tree.Keys() {
				plugins[key] = tree.Get(key)
			}
		}
	}

	pluginsDir := filepath.Join(repoRoot, "plugins")
	filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			relPath, _ := filepath.Rel(repoRoot, path)
			name := strings.TrimSuffix(filepath.Base(path), ".toml")
			pluginInfo := make(map[string]interface{})
			pluginInfo["path"] = relPath
			pluginInfo["version"] = getPluginVersion(path)
			plugins[name] = pluginInfo
		}
		return nil
	})
	catalog.Set("plugins", plugins)
}

func updateTemplates(catalog *toml.Tree, repoRoot string) {
	templates := make(map[string]interface{})
	if templatesTree := catalog.Get("templates"); templatesTree != nil {
		if tree, ok := templatesTree.(*toml.Tree); ok {
			for _, key := range tree.Keys() {
				templates[key] = tree.Get(key)
			}
		}
	}

	templatesDir := filepath.Join(repoRoot, "templates")
	filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			relPath, _ := filepath.Rel(repoRoot, path)
			name := strings.TrimSuffix(filepath.Base(path), ".toml")
			templateInfo := make(map[string]interface{})
			templateInfo["path"] = relPath
			templateInfo["version"] = getTemplateVersion(path)
			templates[name] = templateInfo
		}
		return nil
	})
	catalog.Set("templates", templates)
}

func getPluginVersion(path string) string {
	// Implement logic to extract plugin version from the TOML file
	// This is a placeholder implementation
	return "0.1.0"
}

func getTemplateVersion(path string) string {
	// Implement logic to extract template version from the TOML file
	// This is a placeholder implementation
	return "0.1.0"
}

func incrementVersion(catalog *toml.Tree) {
	catalogInfo := catalog.Get("catalog").(*toml.Tree)
	versionInterface := catalogInfo.Get("version")
	if versionInterface == nil {
		catalogInfo.Set("version", "0.1.0")
		return
	}

	version, ok := versionInterface.(string)
	if !ok {
		catalogInfo.Set("version", "0.1.0")
		return
	}

	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		catalogInfo.Set("version", "0.1.0")
		return
	}

	patch := parts[2]
	newPatch := fmt.Sprintf("%d", atoi(patch)+1)
	newVersion := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], newPatch)
	catalogInfo.Set("version", newVersion)
}

func updateLastUpdated(catalog *toml.Tree) {
	lastUpdated := make(map[string]string)
	lastUpdated["date"] = time.Now().Format(time.RFC3339)
	lastUpdated["commit_hash"] = getLatestCommitHash()
	catalog.Set("catalog.last_updated", lastUpdated)
}

func getLatestCommitHash() string {
	// Implement logic to get the latest commit hash
	// This is a placeholder implementation
	return "abc123"
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
