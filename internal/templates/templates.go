// Package templates provides utilities for loading and rendering HTML templates from files.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

// Loader handles loading and rendering templates from a directory.
type Loader struct {
	dir string
}

// NewLoader creates a new template loader for the given directory.
func NewLoader(dir string) *Loader {
	return &Loader{dir: dir}
}

// Load reads and parses a template file.
func (l *Loader) Load(name string) (*template.Template, error) {
	path := filepath.Join(l.dir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	return tmpl, nil
}

// LoadWithFuncs reads and parses a template file with custom functions.
func (l *Loader) LoadWithFuncs(name string, funcs template.FuncMap) (*template.Template, error) {
	path := filepath.Join(l.dir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Funcs(funcs).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	return tmpl, nil
}

// Render loads a template and executes it with the given data.
func (l *Loader) Render(name string, data any) (string, error) {
	tmpl, err := l.Load(name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

// RenderWithFuncs loads a template with custom functions and executes it.
func (l *Loader) RenderWithFuncs(name string, data any, funcs template.FuncMap) (string, error) {
	tmpl, err := l.LoadWithFuncs(name, funcs)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

// EmbeddedLoader handles loading templates from an embedded filesystem.
type EmbeddedLoader struct {
	fs embed.FS
}

// NewEmbeddedLoader creates a new template loader for an embedded filesystem.
func NewEmbeddedLoader(fs embed.FS) *EmbeddedLoader {
	return &EmbeddedLoader{fs: fs}
}

// Load reads and parses a template from the embedded filesystem.
func (l *EmbeddedLoader) Load(name string) (*template.Template, error) {
	content, err := l.fs.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read embedded template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	return tmpl, nil
}

// LoadWithFuncs reads and parses a template with custom functions from the embedded filesystem.
func (l *EmbeddedLoader) LoadWithFuncs(name string, funcs template.FuncMap) (*template.Template, error) {
	content, err := l.fs.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read embedded template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Funcs(funcs).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	return tmpl, nil
}

// Render loads a template from the embedded filesystem and executes it.
func (l *EmbeddedLoader) Render(name string, data any) (string, error) {
	tmpl, err := l.Load(name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

// RenderWithFuncs loads a template with custom functions and executes it.
func (l *EmbeddedLoader) RenderWithFuncs(name string, data any, funcs template.FuncMap) (string, error) {
	tmpl, err := l.LoadWithFuncs(name, funcs)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}

	return buf.String(), nil
}
