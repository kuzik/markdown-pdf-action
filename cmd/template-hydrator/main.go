package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kuzik/pandoc-latex-docker/internal/images"
	"github.com/kuzik/pandoc-latex-docker/internal/markdown"
	"github.com/kuzik/pandoc-latex-docker/internal/pdf"
	"github.com/kuzik/pandoc-latex-docker/internal/templates"
)

//go:embed template.html
var templateFS embed.FS

type pageData struct {
	Title   string
	Content template.HTML
}

var (
	wrapperLoader *templates.EmbeddedLoader
	mdConverter   *markdown.Converter
)

func init() {
	wrapperLoader = templates.NewEmbeddedLoader(templateFS)
	mdConverter = markdown.DefaultConverter()
}

func main() {
	var (
		templatePath string
		dataPath     string
		outputDir    string
		imagesDir    string
	)

	flag.StringVar(&templatePath, "template", "", "Path to the .html or .md template file")
	flag.StringVar(&dataPath, "data", "", "Path to the .json data file")
	flag.StringVar(&outputDir, "output", "", "Path to the directory where PDFs will be saved")
	flag.StringVar(&imagesDir, "images", "", "Base path for resolving image paths (defaults to template directory)")
	flag.Parse()

	if templatePath == "" || dataPath == "" || outputDir == "" {
		log.Fatal("--template, --data, and --output must all be provided")
	}

	// Determine base directory for images
	imageBasePath := imagesDir
	if imageBasePath == "" {
		imageBasePath = filepath.Dir(templatePath)
	}

	// Load template
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		log.Fatalf("Failed to read template: %v", err)
	}

	// Parse template
	tmpl, err := template.New("document").Parse(string(tmplContent))
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Determine if template is markdown
	isMarkdown := strings.HasSuffix(strings.ToLower(templatePath), ".md")

	// Load JSON data
	dataContent, err := os.ReadFile(dataPath)
	if err != nil {
		log.Fatalf("Failed to read data file: %v", err)
	}

	// Parse JSON as map - values can be any structure (nested objects, arrays, etc.)
	var dataMap map[string]any
	if err := json.Unmarshal(dataContent, &dataMap); err != nil {
		log.Fatalf("Failed to parse JSON data: %v", err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Process each entry
	for name, data := range dataMap {
		if err := renderDocument(tmpl, data, name, outputDir, isMarkdown, imageBasePath); err != nil {
			log.Printf("Failed to render %s: %v", name, err)
			continue
		}
		log.Printf("Rendered: %s.pdf", name)
	}
}

// renderDocument renders a single document from template and data
func renderDocument(tmpl *template.Template, data any, name, outputDir string, isMarkdown bool, imageBasePath string) error {
	// Execute template with data
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	content := buf.String()

	// Convert markdown to HTML if needed
	if isMarkdown {
		html, err := mdConverter.ToHTML([]byte(content))
		if err != nil {
			return fmt.Errorf("convert markdown: %w", err)
		}
		content = html
	}

	// Embed images as base64 data URLs
	content, err := images.EmbedImagesAsBase64(content, imageBasePath)
	if err != nil {
		return fmt.Errorf("embed images: %w", err)
	}

	var fullHTML string

	// Markdown templates always need wrapping for styles
	// HTML templates with their own doctype/head/style are used directly
	if !isMarkdown && isCompleteHTMLDocument(content) {
		// Use the template's HTML directly without wrapping
		fullHTML = content
	} else {
		// Wrap in styled HTML template
		title := name
		if dataMap, ok := data.(map[string]any); ok {
			if t, ok := dataMap["Title"].(string); ok {
				title = t
			}
		}

		fullHTML, err = wrapHTML(content, title)
		if err != nil {
			return fmt.Errorf("wrap HTML: %w", err)
		}
	}

	// Generate PDF
	outputPath := filepath.Join(outputDir, name+".pdf")
	if err := pdf.FromHTML(fullHTML, outputPath); err != nil {
		return fmt.Errorf("generate PDF: %w", err)
	}

	return nil
}

// isCompleteHTMLDocument checks if the content appears to be a complete HTML document
// with its own styling (doctype, html tag, head section, or style tags)
func isCompleteHTMLDocument(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "<!doctype") ||
		strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<head") ||
		strings.Contains(lower, "<style")
}

// wrapHTML wraps content in a styled HTML template
func wrapHTML(content, title string) (string, error) {
	data := pageData{
		Title:   title,
		Content: template.HTML(content),
	}
	return wrapperLoader.Render("template.html", data)
}
