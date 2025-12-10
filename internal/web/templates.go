package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*
var templateFS embed.FS

func getTemplates() *template.Template {
	// Parse templates from the embedded file system
	// The pattern needs to match the embedded path structure
	// Since we embed "templates/*", and the file is in "web" package but file structure is internal/web/templates...
	// Wait, the embed directive uses paths relative to the source file.
	// If templates.go is in internal/web/, and templates are in internal/web/templates/, then `templates/*` is correct.
	return template.Must(template.ParseFS(templateFS, "templates/*.html"))
}
