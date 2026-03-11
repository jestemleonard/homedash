package renderer

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Override directories checked before default templates
var overrideDirs = []string{"overrides/templates", "custom/templates"}

// TemplateCache holds parsed templates
type TemplateCache struct {
	templates    map[string]*template.Template
	templatesDir string
	funcs        template.FuncMap
}

// NewTemplateCache creates a new template cache
func NewTemplateCache(templatesDir string) (*TemplateCache, error) {
	tc := &TemplateCache{
		templates:    make(map[string]*template.Template),
		templatesDir: templatesDir,
		funcs:        TemplateFuncs(),
	}

	if err := tc.loadTemplates(); err != nil {
		return nil, err
	}

	return tc, nil
}

// resolveFile returns the path to the template file, checking override dirs first.
// subPath is relative to the templates root, e.g. "layouts/base.html".
func (tc *TemplateCache) resolveFile(subPath string) string {
	for _, dir := range overrideDirs {
		candidate := filepath.Join(dir, subPath)
		if _, err := os.Stat(candidate); err == nil {
			slog.Info("using template override", "path", candidate)
			return candidate
		}
	}
	return filepath.Join(tc.templatesDir, subPath)
}

// loadTemplates parses all templates from the templates directory
func (tc *TemplateCache) loadTemplates() error {
	// Get base layout (with override support)
	layoutPath := tc.resolveFile("layouts/base.html")
	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		return fmt.Errorf("base layout not found: %s", layoutPath)
	}

	// Load all component templates (default dir first, then apply overrides)
	componentsDir := filepath.Join(tc.templatesDir, "components")
	componentFiles, err := tc.getTemplateFiles(componentsDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Resolve overrides for each component
	resolvedComponents := make([]string, len(componentFiles))
	for i, comp := range componentFiles {
		rel, _ := filepath.Rel(tc.templatesDir, comp)
		resolvedComponents[i] = tc.resolveFile(rel)
	}

	// Load page templates
	pagesDir := filepath.Join(tc.templatesDir, "pages")
	pageFiles, err := tc.getTemplateFiles(pagesDir)
	if err != nil {
		return err
	}

	// Parse each page template with base layout and components
	for _, page := range pageFiles {
		name := filepath.Base(page)
		name = strings.TrimSuffix(name, ".html")

		// Resolve override for the page itself
		rel, _ := filepath.Rel(tc.templatesDir, page)
		resolvedPage := tc.resolveFile(rel)

		// Create template set: base + components + page
		files := []string{layoutPath}
		files = append(files, resolvedComponents...)
		files = append(files, resolvedPage)

		tmpl, err := template.New(filepath.Base(layoutPath)).
			Funcs(tc.funcs).
			ParseFiles(files...)
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", name, err)
		}

		tc.templates[name] = tmpl
	}

	// Also parse components individually for partial rendering
	for _, comp := range resolvedComponents {
		name := "components/" + strings.TrimSuffix(filepath.Base(comp), ".html")
		tmpl, err := template.New(filepath.Base(comp)).
			Funcs(tc.funcs).
			ParseFiles(comp)
		if err != nil {
			return fmt.Errorf("parsing component %s: %w", name, err)
		}
		tc.templates[name] = tmpl
	}

	return nil
}

// getTemplateFiles returns all .html files in a directory
func (tc *TemplateCache) getTemplateFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// Get returns a parsed template by name
func (tc *TemplateCache) Get(name string) (*template.Template, error) {
	tmpl, ok := tc.templates[name]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return tmpl, nil
}

