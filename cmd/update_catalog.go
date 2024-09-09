package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
)

func main() {
	catalog, err := loadCatalog()
	if err != nil {
		fmt.Println("Error loading catalog:", err)
		os.Exit(1)
	}

	updatePlugins(catalog)
	updateTemplates(catalog)
	incrementVersion(catalog)

	err = saveCatalog(catalog)
	if err != nil {
		fmt.Println("Error saving catalog:", err)
		os.Exit(1)
	}

	fmt.Println("Catalog updated successfully")
}

func loadCatalog() (*toml.Tree, error) {
	data, err := ioutil.ReadFile("gitspace-catalog.toml")
	if err != nil {
		return nil, err
	}
	return toml.LoadBytes(data)
}

func updatePlugins(catalog *toml.Tree) {
	plugins := make(map[string]interface{})
	filepath.Walk("plugins", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			name := strings.TrimSuffix(filepath.Base(path), ".toml")
			plugins[name] = path
		}
		return nil
	})
	catalog.Set("plugins", plugins)
}

func updateTemplates(catalog *toml.Tree) {
	templates := make(map[string]interface{})
	filepath.Walk("templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			name := strings.TrimSuffix(filepath.Base(path), ".toml")
			templates[name] = path
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

func saveCatalog(catalog *toml.Tree) error {
	return ioutil.WriteFile("gitspace-catalog.toml", []byte(catalog.String()), 0644)
}
