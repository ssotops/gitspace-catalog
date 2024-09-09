package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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
	err := filepath.Walk("plugins", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			name := filepath.Base(path)
			plugins[name] = path
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking plugins directory:", err)
		return
	}
	catalog.Set("plugins", plugins)
}

func updateTemplates(catalog *toml.Tree) {
	templates := make(map[string]interface{})
	err := filepath.Walk("templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".toml" {
			name := filepath.Base(path)
			templates[name] = path
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking templates directory:", err)
		return
	}
	catalog.Set("templates", templates)
}

func incrementVersion(catalog *toml.Tree) {
	version := catalog.Get("catalog.version").(string)
	// Implement version incrementing logic here
	// For simplicity, we're just appending a "+" to the version
	newVersion := version + "+"
	catalog.Set("catalog.version", newVersion)
}

func saveCatalog(catalog *toml.Tree) error {
	return ioutil.WriteFile("gitspace-catalog.toml", []byte(catalog.String()), 0644)
}
