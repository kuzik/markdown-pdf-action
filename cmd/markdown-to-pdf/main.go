package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	// CSS is optional extra styling injected after the base stylesheet, so it
	// overrides defaults. It may be either a path to a .css file (relative to
	// the working directory) or a literal block of CSS written inline.
	CSS string `yaml:"css"`
}

type renderConfig struct {
	mdPath  string
	outPath string
	baseDir string
	css     string
}

type pageData struct {
	Title   string
	Content template.HTML
	CSS     template.CSS
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
	var configYAML string
	flag.StringVar(&configYAML, "config", "", "YAML config string describing render jobs")
	flag.Parse()

	if configYAML == "" {
		log.Fatal("--config must be provided")
	}

	jobs, err := parseConfig([]byte(configYAML))
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	executeJobs(jobs)
}

// parseConfig parses YAML config bytes into jobs
func parseConfig(cfgBytes []byte) ([]job, error) {
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

	css := resolveCSS(j.CSS)

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
			css:     css,
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

	return renderCombinedMarkdown(combined, j.Output, baseDir, resolveCSS(j.CSS))
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

	// Select the markdown files to combine: README.md files when present
	// (preserving folder-based grouping), otherwise all matched markdown files.
	files := selectCombineFiles(matches)
	if len(files) == 0 {
		return fmt.Errorf("no markdown files found for %s", j.Source)
	}

	// Combine with a page break before each section, converting markdown to HTML
	// for each file individually so images resolve relative to that file's directory.
	combined, err := combineMarkdownAsHTML(files)
	if err != nil {
		return err
	}

	return renderCombinedHTML(combined, j.Output, resolveCSS(j.CSS))
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

// selectCombineFiles chooses which markdown files to combine, sorted by path.
// When any README.md files are present it uses only those (preserving the
// folder-per-section convention); otherwise it falls back to every matched
// markdown file, so flat collections like assessment/01.md..30.md also combine.
func selectCombineFiles(files []string) []string {
	var readmes, markdown []string
	for _, f := range files {
		if filepath.Base(f) == "README.md" {
			readmes = append(readmes, f)
		}
		if strings.EqualFold(filepath.Ext(f), ".md") {
			markdown = append(markdown, f)
		}
	}

	selected := markdown
	if len(readmes) > 0 {
		selected = readmes
	}

	sort.Strings(selected)
	return selected
}

// sectionID returns a stable anchor id for a combined section. README.md files
// are identified by their parent folder name; flat files by their base name
// without the extension (so assessment/01.md becomes "01").
func sectionID(path string) string {
	if filepath.Base(path) == "README.md" {
		return filepath.Base(filepath.Dir(path))
	}
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
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

// combineMarkdownAsHTML converts each markdown file to HTML (with images
// embedded) and combines them, inserting a page break before each section so
// every source file starts on its own page.
func combineMarkdownAsHTML(files []string) (string, error) {
	var htmlParts []string

	for _, file := range files {
		folder := filepath.Dir(file)

		// Read markdown content
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Warning: failed to read %s: %v", file, err)
			continue
		}

		// Convert markdown to HTML
		htmlBody, err := mdConverter.ToHTML(content)
		if err != nil {
			log.Printf("Warning: failed to convert markdown %s: %v", file, err)
			continue
		}

		// Embed images relative to this file's directory
		htmlWithImages, err := images.EmbedImagesAsBase64(htmlBody, folder)
		if err != nil {
			log.Printf("Warning: failed to embed images for %s: %v", file, err)
			// Continue anyway with non-embedded images
			htmlWithImages = htmlBody
		}

		// Prefix each section with a page break anchor so it starts a new page
		htmlParts = append(htmlParts, fmt.Sprintf("<div id=\"%s\" style=\"page-break-before: always; visibility:hidden\"></div>\n%s", sectionID(file), htmlWithImages))
	}

	return strings.Join(htmlParts, "\n\n"), nil
}

// renderCombinedHTML wraps combined HTML content and renders it to PDF
func renderCombinedHTML(htmlContent, outputPath, css string) error {
	// Wrap in styled HTML template
	fullHTML, err := wrapHTML(htmlContent, "Combined", css)
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
func renderCombinedMarkdown(content, outputPath, baseDir, css string) error {
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
		css:     css,
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
	htmlContent, err := wrapHTML(htmlWithImages, filepath.Base(cfg.mdPath), cfg.css)
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

// wrapHTML wraps HTML content in a styled template. The css argument holds
// extra stylesheet text (already resolved from a file or inline value) that is
// injected after the base styles so it can override them.
func wrapHTML(content, title, css string) (string, error) {
	data := pageData{
		Title:   title,
		Content: template.HTML(content),
		CSS:     template.CSS(css),
	}

	return tmplLoader.Render("template.html", data)
}

// resolveCSS turns a job's css value into stylesheet text. When the value names
// an existing file its contents are read; otherwise the value is treated as
// literal inline CSS. An empty value yields an empty string (no extra styles).
func resolveCSS(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	if info, err := os.Stat(value); err == nil && !info.IsDir() {
		content, err := os.ReadFile(value)
		if err != nil {
			log.Printf("Warning: failed to read css file %s: %v", value, err)
			return ""
		}
		return string(content)
	}

	// Not a file path — treat the value itself as CSS.
	return value
}
