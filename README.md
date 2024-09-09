# Gitspace Extension Catalog

Official `ssotops` catalog for `gitspace` extensions, including plugins and templates.

## Structure

- `plugins/`: Contains all available plugins for Gitspace.
- `templates/`: Contains all available templates for Gitspace.

## Using Extensions

### Plugins

To use a plugin, you can install it using the Gitspace CLI:

```
gitspace plugin install github.com/ssotops/catalog/plugins/PLUGIN_NAME
```

Replace `PLUGIN_NAME` with the name of the plugin you want to install.

### Templates

To use a template, you can reference it in your Gitspace configuration:

```toml
[template]
source = "github.com/ssotops/catalog/templates/TEMPLATE_NAME"
```

Replace `TEMPLATE_NAME` with the name of the template you want to use.
