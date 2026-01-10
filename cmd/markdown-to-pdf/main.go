package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kuzik/pandoc-latex-docker/internal/images"
	"github.com/kuzik/pandoc-latex-docker/internal/markdown"
	"github.com/kuzik/pandoc-latex-docker/internal/pdf"
	"github.com/kuzik/pandoc-latex-docker/internal/templates"
	"github.com/kuzik/pandoc-latex-docker/internal/ziputil"
	"gopkg.in/yaml.v3"
)

//go:embed template.html
var templateFS embed.FS

type job struct {
	Source string `yaml:"source"`
	Output string `yaml:"output"`
	Type   string `yaml:"type"` // single | subfolders | combine
}

type renderConfig struct {
	mdPath  string
	outPath string
	baseDir string
}

type pageData struct {
	Title   string
	Content template.HTML
}

var (
	tmplLoader  *templates.EmbeddedLoader
	mdConverter *markdown.Converter
)

func init() {
	tmplLoader = templates.NewEmbeddedLoader(templateFS)
	mdConverter = markdown.DefaultConverter()
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "render.yaml", "Path to YAML config describing render jobs")
	flag.Parse()

	jobs, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	executeJobs(jobs)
}

// loadConfig reads and parses the YAML configuration file
func loadConfig(configPath string) ([]job, error) {
	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var jobs []job
	if err := yaml.Unmarshal(cfgBytes, &jobs); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	return jobs, nil
}

// executeJobs processes all jobs from the configuration
func executeJobs(jobs []job) {
	for _, j := range jobs {
		if err := executeJob(j); err != nil {
			log.Printf("Job failed (%s %s): %v", j.Type, j.Source, err)
		}
	}
}

// executeJob routes a job to the appropriate handler based on its type
func executeJob(j job) error {
	switch j.Type {
	case "subfolders":
		return renderSubfolders(j)
	case "single":
		return renderSingle(j)
	case "combine":
		return renderCombine(j)
	default:
		return fmt.Errorf("unknown job type %q", j.Type)
	}
}

// renderSubfolders renders each README.md in matched subdirectories as a separate PDF
func renderSubfolders(j job) error {
	matches, err := findMatches(j.Source)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(j.Output, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	for _, m := range matches {
		if filepath.Base(m) != "README.md" {
			continue
		}

		folder := filepath.Dir(m)
		folderName := filepath.Base(folder)
		outPDF := filepath.Join(j.Output, folderName+".pdf")

		if err := renderMarkdownToPDF(renderConfig{
			mdPath:  m,
			outPath: outPDF,
			baseDir: folder,
		}); err != nil {
			log.Printf("Render %s: %v", m, err)
			continue
		}

		// Create source zip if src directory exists
		if err := zipSourceIfExists(folder, j.Output, folderName); err != nil {
			log.Printf("Zip src %s: %v", folder, err)
		}
	}

	return nil
}

// renderSingle combines multiple markdown files into a single PDF
func renderSingle(j job) error {
	matches, err := findMatches(j.Source)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(j.Output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Combine all matched markdown files
	combined, err := combineMarkdownFiles(matches, "\n\n")
	if err != nil {
		return err
	}

	// Determine base directory for image resolution
	baseDir := ""
	if len(matches) > 0 {
		baseDir = filepath.Dir(matches[0])
	}

	return renderCombinedMarkdown(combined, j.Output, baseDir)
}

// renderCombine merges multiple README.md files with folder headers into a single PDF
func renderCombine(j job) error {
	matches, err := findMatches(j.Source)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(j.Output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Filter only README.md files
	readmes := filterREADMEs(matches)
	if len(readmes) == 0 {
		return fmt.Errorf("no README.md files found for %s", j.Source)
	}

	// Combine with folder headers, converting markdown to HTML for each README individually
	// This ensures images are resolved relative to each README's directory
	combined, err := combineREADMEsAsHTML(readmes)
	if err != nil {
		return err
	}

	return renderCombinedHTML(combined, j.Output)
}

// findMatches finds all files matching the glob pattern
func findMatches(pattern string) ([]string, error) {
	matches, err := doublestar.Glob(os.DirFS("."), pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matches for %s", pattern)
	}
	return matches, nil
}

// filterREADMEs returns only README.md files from the list
func filterREADMEs(files []string) []string {
	var readmes []string
	for _, f := range files {
		if filepath.Base(f) == "README.md" {
			readmes = append(readmes, f)
		}
	}
	return readmes
}

// combineMarkdownFiles reads and combines multiple markdown files
func combineMarkdownFiles(files []string, separator string) (string, error) {
	var parts []string
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", f, err)
		}
		parts = append(parts, string(content))
	}
	return strings.Join(parts, separator), nil
}

// combineREADMEsAsHTML converts each README to HTML (with images embedded) and combines them
func combineREADMEsAsHTML(readmes []string) (string, error) {
	var htmlParts []string

	for _, readme := range readmes {
		folder := filepath.Dir(readme)
		folderName := filepath.Base(folder)

		// Read markdown content
		content, err := os.ReadFile(readme)
		if err != nil {
			log.Printf("Warning: failed to read %s: %v", readme, err)
			continue
		}

		// Convert markdown to HTML
		htmlBody, err := mdConverter.ToHTML(content)
		if err != nil {
			log.Printf("Warning: failed to convert markdown %s: %v", readme, err)
			continue
		}

		// Embed images relative to this README's directory
		htmlWithImages, err := images.EmbedImagesAsBase64(htmlBody, folder)
		if err != nil {
			log.Printf("Warning: failed to embed images for %s: %v", readme, err)
			// Continue anyway with non-embedded images
			htmlWithImages = htmlBody
		}

		// Add folder name as HTML header and the content
		htmlParts = append(htmlParts, fmt.Sprintf("<div id=\"%s\" style=\"page-break-before: always; visibility:hidden\"></div>\n%s", folderName, htmlWithImages))
	}

	return strings.Join(htmlParts, "\n\n"), nil
}

// renderCombinedHTML wraps combined HTML content and renders it to PDF
func renderCombinedHTML(htmlContent, outputPath string) error {
	// Wrap in styled HTML template
	fullHTML, err := wrapHTML(htmlContent, "Combined")
	if err != nil {
		return fmt.Errorf("wrap HTML: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Convert HTML to PDF
	if err := pdf.FromHTML(fullHTML, outputPath); err != nil {
		return fmt.Errorf("convert to PDF: %w", err)
	}

	log.Printf("Rendered: %s", outputPath)
	return nil
}

// renderCombinedMarkdown writes combined markdown to a temp file and renders it
func renderCombinedMarkdown(content, outputPath, baseDir string) error {
	tmpFile, err := os.CreateTemp("", "combined-*.md")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	return renderMarkdownToPDF(renderConfig{
		mdPath:  tmpFile.Name(),
		outPath: outputPath,
		baseDir: baseDir,
	})
}

// zipSourceIfExists creates a zip of the src directory if it exists
func zipSourceIfExists(folder, outputDir, baseName string) error {
	srcDir := filepath.Join(folder, "src")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}

	zipName := filepath.Join(outputDir, baseName+"_src.zip")
	return ziputil.CreateFromFolder(srcDir, zipName)
}

// renderMarkdownToPDF converts a markdown file to PDF
func renderMarkdownToPDF(cfg renderConfig) error {
	// Read markdown source
	src, err := os.ReadFile(cfg.mdPath)
	if err != nil {
		return fmt.Errorf("read markdown: %w", err)
	}

	// Convert markdown to HTML
	htmlBody, err := mdConverter.ToHTML(src)
	if err != nil {
		return fmt.Errorf("convert markdown: %w", err)
	}

	// Determine base directory for resolving images
	baseDir := cfg.baseDir
	if baseDir == "" {
		baseDir = filepath.Dir(cfg.mdPath)
	}

	// Embed images as base64 data URLs
	htmlWithImages, err := images.EmbedImagesAsBase64(htmlBody, baseDir)
	if err != nil {
		return fmt.Errorf("embed images: %w", err)
	}

	// Wrap in styled HTML template
	htmlContent, err := wrapHTML(htmlWithImages, filepath.Base(cfg.mdPath))
	if err != nil {
		return fmt.Errorf("wrap HTML: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.outPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Convert HTML to PDF
	if err := pdf.FromHTML(htmlContent, cfg.outPath); err != nil {
		return fmt.Errorf("convert to PDF: %w", err)
	}

	log.Printf("Rendered: %s", cfg.outPath)
	return nil
}

// wrapHTML wraps HTML content in a styled template
func wrapHTML(content, title string) (string, error) {
	data := pageData{
		Title:   title,
		Content: template.HTML(content),
	}

	return tmplLoader.Render("template.html", data)
}
