package main

import (
	"embed"
	"fmt"
	"io/fs"
	"text/template"
)

const (
	templatesDir = "templates"
)

var (
	//go:embed templates
	files     embed.FS
	templates map[string]*template.Template
)

func loadTemplates() error {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}

	tmplFiles, err := fs.ReadDir(files, templatesDir)
	if err != nil {
		return fmt.Errorf("read templates dir: %w", err)
	}

	for _, tmpl := range tmplFiles {
		if tmpl.IsDir() {
			continue
		}

		pt, err := template.ParseFS(files, templatesDir+"/"+tmpl.Name())
		if err != nil {
			return fmt.Errorf("parse template %s: %w", tmpl.Name(), err)
		}

		templates[tmpl.Name()] = pt
	}

	return nil
}
