package web

import (
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*
var templateFS embed.FS

// Renderer handles all view-related logic and template rendering
type Renderer struct {
	tmpl *template.Template
}

func NewRenderer() (
	*Renderer,
	error,
) {
	tmpl := template.New("base")

	tmpl, err := tmpl.ParseFS(templateFS, "templates/*")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return &Renderer{tmpl: tmpl}, nil
}
