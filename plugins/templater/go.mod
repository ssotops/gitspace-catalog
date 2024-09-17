module github.com/ssotops/gitspace-catalog/plugins/templater

go 1.23.1

require (
    github.com/charmbracelet/huh v0.6.0
    github.com/charmbracelet/log v0.4.0
    github.com/ssotops/gitspace/gsplugin v0.0.0-00010101000000-000000000000
)

replace github.com/ssotops/gitspace/gsplugin => ../../gitspace/gsplugin

replace github.com/ssotops/gitspace => ../../gitspace
