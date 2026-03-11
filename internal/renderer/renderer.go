package renderer

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// Renderer handles template rendering
type Renderer struct {
	cache *TemplateCache
}

// New creates a new Renderer
func New(templatesDir string) (*Renderer, error) {
	cache, err := NewTemplateCache(templatesDir)
	if err != nil {
		return nil, err
	}

	return &Renderer{cache: cache}, nil
}

// RenderPage renders a full page template to the response writer
func (r *Renderer) RenderPage(w http.ResponseWriter, name string, data any) error {
	tmpl, err := r.cache.Get(name)
	if err != nil {
		return err
	}

	// Render to buffer first to catch errors
	buf := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(buf, "base", data); err != nil {
		return fmt.Errorf("executing template %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = buf.WriteTo(w)
	return err
}

// RenderComponent renders a component template (for HTMX partial updates)
func (r *Renderer) RenderComponent(w http.ResponseWriter, name string, data any) error {
	tmpl, err := r.cache.Get("components/" + name)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
		return fmt.Errorf("executing component %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = buf.WriteTo(w)
	return err
}

// RenderComponentToWriter renders a component to any writer
func (r *Renderer) RenderComponentToWriter(w io.Writer, name string, data any) error {
	tmpl, err := r.cache.Get("components/" + name)
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, name, data)
}
