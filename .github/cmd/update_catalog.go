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
	fmt.Println("Starting catalog update process...")
	catalogPath := filepath.Join(repoRoot, "gitspace-catalog.toml")
	fmt.Printf("Catalog path: %s\n", catalogPath)

	catalog, err := loadCatalog(catalogPath)
	if err != nil {
		return fmt.Errorf("error loading catalog: %w", err)
	}

	fmt.Println("Loaded catalog content:")
	fmt.Println(catalog.String())

	preserveCatalogInfo(catalog)
	updatePlugins(catalog, repoRoot)
	updateTemplates(catalog, repoRoot)
	incrementVersion(catalog)
	updateLastUpdated(catalog)

	// Convert absolute paths to relative paths
	convertToRelativePaths(catalog, repoRoot)

	fmt.Println("Updated catalog content:")
	updatedContent := formatTomlTree(catalog)
	fmt.Println(updatedContent)

	if updatedContent == "" {
		return fmt.Errorf("updated catalog content is empty, aborting save to prevent data loss")
	}

	err = saveCatalog(updatedContent, catalogPath)
	if err != nil {
		return fmt.Errorf("error saving catalog: %w", err)
	}

	fmt.Println("Catalog updated successfully")
	return nil
}

func convertToRelativePaths(catalog *toml.Tree, repoRoot string) {
	convertSection := func(sectionName string) {
		if section, ok := catalog.Get(sectionName).(*toml.Tree); ok {
			for _, key := range section.Keys() {
				if item, ok := section.Get(key).(*toml.Tree); ok {
					if path, ok := item.Get("path").(string); ok {
						relPath, err := filepath.Rel(repoRoot, path)
						if err == nil {
							item.Set("path", relPath)
						}
					}
				}
			}
		}
	}

	convertSection("plugins")
	convertSection("templates")
}

func loadCatalog(path string) (*toml.Tree, error) {
	fmt.Printf("Attempting to load catalog from: %s\n", path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("Catalog file does not exist, creating a new one")
		return toml.Load(`[catalog]
name = "Gitspace Official Catalog"
description = "Official catalog of plugins and templates for Gitspace"
version = "0.1.0"

[plugins]

[templates]
`)
	}
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading catalog file: %w", err)
	}
	fmt.Printf("Loaded catalog content:\n%s\n", string(content))
	return toml.Load(string(content))
}

func preserveCatalogInfo(catalog *toml.Tree) {
	fmt.Println("Preserving catalog info...")
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
	catalog.Set("catalog", catalogInfo)
	fmt.Printf("Preserved catalog info: %v\n", catalogInfo)
}

func updatePlugins(catalog *toml.Tree, repoRoot string) {
	fmt.Println("Updating plugins...")
	plugins := make(map[string]interface{})
	pluginsDir := filepath.Join(repoRoot, "plugins")
	fmt.Printf("Plugins directory: %s\n", pluginsDir)

	err := filepath.Walk(pluginsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if info.IsDir() && info.Name() != "plugins" {
			fmt.Printf("Found plugin directory: %s\n", info.Name())
			pluginInfo, err := loadPluginInfo(path)
			if err == nil {
				plugins[info.Name()] = pluginInfo
				fmt.Printf("Added plugin %s: %v\n", info.Name(), pluginInfo)
			} else {
				fmt.Printf("Error loading plugin info for %s: %v\n", info.Name(), err)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking plugins directory: %v\n", err)
	}

	pluginsTree, _ := toml.TreeFromMap(plugins)
	catalog.Set("plugins", pluginsTree)
	fmt.Printf("Updated plugins: %v\n", plugins)
}

func loadPluginInfo(pluginDir string) (map[string]interface{}, error) {
	tomlPath := filepath.Join(pluginDir, "gitspace-plugin.toml")
	fmt.Printf("Loading plugin info from: %s\n", tomlPath)
	tree, err := toml.LoadFile(tomlPath)
	if err != nil {
		return nil, fmt.Errorf("error loading plugin TOML: %w", err)
	}

	info := make(map[string]interface{})
	info["version"] = tree.Get("plugin.version")
	info["description"] = tree.Get("plugin.description")
	info["path"] = pluginDir
	fmt.Printf("Loaded plugin info: %v\n", info)
	return info, nil
}

func updateTemplates(catalog *toml.Tree, repoRoot string) {
	fmt.Println("Updating templates...")
	templates := make(map[string]interface{})
	templatesDir := filepath.Join(repoRoot, "templates")
	fmt.Printf("Templates directory: %s\n", templatesDir)

	err := filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}
		if info.IsDir() && info.Name() != "templates" {
			fmt.Printf("Found template directory: %s\n", info.Name())
			templateInfo, err := loadTemplateInfo(path)
			if err == nil {
				templates[info.Name()] = templateInfo
				fmt.Printf("Added template %s: %v\n", info.Name(), templateInfo)
			} else {
				fmt.Printf("Error loading template info for %s: %v\n", info.Name(), err)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking templates directory: %v\n", err)
	}

	templatesTree, _ := toml.TreeFromMap(templates)
	catalog.Set("templates", templatesTree)
	fmt.Printf("Updated templates: %v\n", templates)
}

func loadTemplateInfo(templateDir string) (map[string]interface{}, error) {
	var tomlPath string
	if _, err := os.Stat(filepath.Join(templateDir, "gitspace-catalog.toml")); err == nil {
		tomlPath = filepath.Join(templateDir, "gitspace-catalog.toml")
	} else {
		tomlPath = filepath.Join(templateDir, "gitspace-plugin.toml")
	}

	fmt.Printf("Loading template info from: %s\n", tomlPath)
	tree, err := toml.LoadFile(tomlPath)
	if err != nil {
		return nil, fmt.Errorf("error loading template TOML: %w", err)
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
	fmt.Printf("Loaded template info: %v\n", info)
	return info, nil
}

func incrementVersion(catalog *toml.Tree) {
	fmt.Println("Incrementing version...")
	catalogInfo := catalog.Get("catalog").(*toml.Tree)
	version := catalogInfo.Get("version").(string)
	parts := strings.Split(version, ".")
	if len(parts) == 3 {
		patch := atoi(parts[2])
		newVersion := fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
		catalogInfo.Set("version", newVersion)
		fmt.Printf("Incremented version from %s to %s\n", version, newVersion)
	} else {
		fmt.Printf("Invalid version format: %s\n", version)
	}
	catalog.Set("catalog", catalogInfo)
}

func updateLastUpdated(catalog *toml.Tree) {
	fmt.Println("Updating last updated info...")
	catalogInfo := catalog.Get("catalog").(*toml.Tree)
	lastUpdated := make(map[string]interface{})
	lastUpdated["date"] = time.Now().Format(time.RFC3339)
	lastUpdated["commit_hash"] = getLatestCommitHash()
	lastUpdatedTree, _ := toml.TreeFromMap(lastUpdated)
	catalogInfo.Set("last_updated", lastUpdatedTree)
	catalog.Set("catalog", catalogInfo)
	fmt.Printf("Updated last updated info: %v\n", lastUpdated)
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

func formatTomlTree(tree *toml.Tree) string {
	var sb strings.Builder

	// Format catalog section
	catalogTree := tree.Get("catalog").(*toml.Tree)
	sb.WriteString("[catalog]\n")
	for _, k := range catalogTree.Keys() {
		v := catalogTree.Get(k)
		if k == "last_updated" {
			sb.WriteString(fmt.Sprintf("%s = %s\n", k, formatLastUpdated(v.(*toml.Tree))))
		} else {
			sb.WriteString(fmt.Sprintf("%s = %q\n", k, v))
		}
	}
	sb.WriteString("\n")

	// Format plugins section
	sb.WriteString("[plugins]\n")
	if pluginsTree := tree.Get("plugins"); pluginsTree != nil {
		for _, plugin := range pluginsTree.(*toml.Tree).Keys() {
			pluginInfo := pluginsTree.(*toml.Tree).Get(plugin).(*toml.Tree)
			sb.WriteString(fmt.Sprintf("[plugins.%s]\n", plugin))
			for _, k := range pluginInfo.Keys() {
				v := pluginInfo.Get(k)
				sb.WriteString(fmt.Sprintf("%s = %q\n", k, v))
			}
			sb.WriteString("\n")
		}
	}

	// Format templates section
	sb.WriteString("[templates]\n")
	if templatesTree := tree.Get("templates"); templatesTree != nil {
		for _, template := range templatesTree.(*toml.Tree).Keys() {
			templateInfo := templatesTree.(*toml.Tree).Get(template).(*toml.Tree)
			sb.WriteString(fmt.Sprintf("[templates.%s]\n", template))
			for _, k := range templateInfo.Keys() {
				v := templateInfo.Get(k)
				sb.WriteString(fmt.Sprintf("%s = %q\n", k, v))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatLastUpdated(lastUpdated *toml.Tree) string {
	return fmt.Sprintf("{ date = %q, commit_hash = %q }",
		lastUpdated.Get("date"),
		lastUpdated.Get("commit_hash"))
}

func saveCatalog(content string, path string) error {
	fmt.Printf("Saving catalog to: %s\n", path)
	fmt.Printf("Catalog content to be saved:\n%s\n", content)
	if content == "" {
		return fmt.Errorf("catalog content is empty, aborting save to prevent data loss")
	}
	return ioutil.WriteFile(path, []byte(content), 0644)
}

func findRepoRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "gitspace-catalog.toml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the root
			if strings.HasSuffix(dir, "gitspace-catalog") {
				// We're likely in the GitHub Actions environment
				return dir
			}
			// If we can't find the root, return the starting directory
			return start
		}
		dir = parent
	}
}
