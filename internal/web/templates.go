package web

import (
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*
var templateFS embed.FS

// Presentation handles all view-related logic and template rendering
type Presentation struct {
	tmpl *template.Template
}

// NewPresentation creates a new Presentation layer
func NewPresentation() (*Presentation, error) {
	tmpl := template.New("base")

	tmpl, err := tmpl.ParseFS(templateFS, "templates/*")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return &Presentation{tmpl: tmpl}, nil
}
