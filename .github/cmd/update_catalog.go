package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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

	updatePlugins(catalog, repoRoot)
	updateTemplates(catalog, repoRoot)
	incrementVersion(catalog)

	err = saveCatalog(catalog, catalogPath)
	if err != nil {
		return fmt.Errorf("error saving catalog: %w", err)
	}

	fmt.Println("Catalog updated successfully")
	return nil
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
	pluginsDir := filepath.Join(repoRoot, "plugins")
	filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			relPath, _ := filepath.Rel(repoRoot, path)
			name := strings.TrimSuffix(filepath.Base(path), ".toml")
			plugins[name] = relPath
		}
		return nil
	})
	catalog.Set("plugins", plugins)
}

func updateTemplates(catalog *toml.Tree, repoRoot string) {
	templates := make(map[string]interface{})
	templatesDir := filepath.Join(repoRoot, "templates")
	filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			relPath, _ := filepath.Rel(repoRoot, path)
			name := strings.TrimSuffix(filepath.Base(path), ".toml")
			templates[name] = relPath
		}
		return nil
	})
	catalog.Set("templates", templates)
}

func incrementVersion(catalog *toml.Tree) {
	version := catalog.Get("catalog.version").(string)
	parts := strings.Split(version, ".")
	if len(parts) == 3 {
		patch := parts[2]
		newPatch := fmt.Sprintf("%d", atoi(patch)+1)
		newVersion := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], newPatch)
		catalog.Set("catalog.version", newVersion)
	}
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
