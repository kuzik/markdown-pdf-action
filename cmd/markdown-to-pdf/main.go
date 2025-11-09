package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

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

	// Combine with folder headers
	combined, err := combineREADMEsWithHeaders(readmes)
	if err != nil {
		return err
	}

	// Use first README's directory as base for images
	baseDir := ""
	if len(readmes) > 0 {
		baseDir = filepath.Dir(readmes[0])
	}

	return renderCombinedMarkdown(combined, j.Output, baseDir)
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

// combineREADMEsWithHeaders combines README files with folder name headers
func combineREADMEsWithHeaders(readmes []string) (string, error) {
	var parts []string
	for _, readme := range readmes {
		folder := filepath.Dir(readme)
		folderName := filepath.Base(folder)

		// Add folder name as header
		parts = append(parts, fmt.Sprintf("# %s\n", folderName))

		// Read and add content
		content, err := os.ReadFile(readme)
		if err != nil {
			log.Printf("Warning: failed to read %s: %v", readme, err)
			continue
		}
		parts = append(parts, string(content))
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
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
	return zipFolder(srcDir, zipName)
}

// renderMarkdownToPDF converts a markdown file to PDF
func renderMarkdownToPDF(cfg renderConfig) error {
	// Read markdown source
	src, err := os.ReadFile(cfg.mdPath)
	if err != nil {
		return fmt.Errorf("read markdown: %w", err)
	}

	// Convert markdown to HTML
	htmlBody, err := markdownToHTML(src)
	if err != nil {
		return fmt.Errorf("convert markdown: %w", err)
	}

	// Determine base directory for resolving images
	baseDir := cfg.baseDir
	if baseDir == "" {
		baseDir = filepath.Dir(cfg.mdPath)
	}

	// Embed images as base64 data URLs
	htmlWithImages, err := embedImagesAsBase64(htmlBody, baseDir)
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
	if err := htmlToPDF(htmlContent, cfg.outPath); err != nil {
		return fmt.Errorf("convert to PDF: %w", err)
	}

	log.Printf("Rendered: %s", cfg.outPath)
	return nil
}

// markdownToHTML converts markdown text to HTML
func markdownToHTML(src []byte) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// embedImagesAsBase64 replaces relative image paths with base64 data URLs
func embedImagesAsBase64(htmlContent, baseDir string) (string, error) {
	imgRegex := regexp.MustCompile(`<img\s+[^>]*src=["']([^"']+)["'][^>]*>`)

	result := imgRegex.ReplaceAllStringFunc(htmlContent, func(imgTag string) string {
		srcPath := extractSrcAttribute(imgTag)
		if srcPath == "" {
			return imgTag
		}

		// Skip data URLs and absolute URLs
		if isAbsoluteOrDataURL(srcPath) {
			return imgTag
		}

		// Convert to data URL
		dataURL, err := imageToDataURL(srcPath, baseDir)
		if err != nil {
			log.Printf("Warning: failed to embed image %s: %v", srcPath, err)
			return imgTag
		}

		return replaceImageSrc(imgTag, dataURL)
	})

	return result, nil
}

// extractSrcAttribute extracts the src value from an img tag
func extractSrcAttribute(imgTag string) string {
	srcRegex := regexp.MustCompile(`src=["']([^"']+)["']`)
	matches := srcRegex.FindStringSubmatch(imgTag)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// isAbsoluteOrDataURL checks if a URL is absolute or a data URL
func isAbsoluteOrDataURL(url string) bool {
	return strings.HasPrefix(url, "data:") ||
		strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://")
}

// imageToDataURL reads an image and converts it to a base64 data URL
func imageToDataURL(srcPath, baseDir string) (string, error) {
	imagePath := filepath.Join(baseDir, srcPath)

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	mimeType := getMimeType(imagePath)
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
}

// getMimeType determines the MIME type from file extension
func getMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := mime.TypeByExtension(ext)

	if mimeType != "" {
		return mimeType
	}

	// Fallback to common types
	mimeTypes := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".webp": "image/webp",
	}

	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}

	return "image/png" // default
}

// replaceImageSrc replaces the src attribute in an img tag
func replaceImageSrc(imgTag, newSrc string) string {
	srcRegex := regexp.MustCompile(`src=["']([^"']+)["']`)
	return srcRegex.ReplaceAllString(imgTag, fmt.Sprintf(`src="%s"`, newSrc))
}

// wrapHTML wraps HTML content in a styled template
func wrapHTML(content, title string) (string, error) {
	tmpl := template.Must(template.New("page").Parse(htmlTemplate))

	var buf bytes.Buffer
	data := struct {
		Title   string
		Content template.HTML
	}{
		Title:   title,
		Content: template.HTML(content),
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// htmlToPDF converts HTML content to PDF using headless Chrome
func htmlToPDF(htmlContent, outPath string) error {
	// Write HTML to temporary file
	tmpFile, err := writeTempHTML(htmlContent)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// Setup Chrome context
	ctx, cancel, err := setupChromeContext()
	if err != nil {
		return err
	}
	defer cancel()

	// Generate PDF
	pdfBuf, err := generatePDF(ctx, tmpFile)
	if err != nil {
		return err
	}

	// Write PDF to output file
	if err := os.WriteFile(outPath, pdfBuf, 0o644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}

	return nil
}

// writeTempHTML writes HTML content to a temporary file
func writeTempHTML(htmlContent string) (string, error) {
	tmpFile, err := os.CreateTemp("", "markdown-*.html")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// setupChromeContext creates a Chrome context with appropriate options
func setupChromeContext() (context.Context, context.CancelFunc, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.Flag("allow-file-access-from-files", true),
		chromedp.Flag("disable-web-security", true),
	)

	if chromeBin := os.Getenv("CHROME_BIN"); chromeBin != "" {
		opts = append(opts, chromedp.ExecPath(chromeBin))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)

	cancel := func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}

	return ctx, cancel, nil
}

// generatePDF uses Chrome to convert HTML file to PDF
func generatePDF(ctx context.Context, htmlPath string) ([]byte, error) {
	var pdfBuf []byte

	err := chromedp.Run(ctx,
		chromedp.Navigate("file://"+htmlPath),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(false).
				WithPaperWidth(8.27).   // A4 width
				WithPaperHeight(11.69). // A4 height
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("chromedp: %w", err)
	}

	return pdfBuf, nil
}

// zipFolder creates a zip archive of a directory
func zipFolder(srcDir, outZip string) error {
	f, err := os.Create(outZip)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(srcDir, path)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}

		rf, err := os.Open(path)
		if err != nil {
			return err
		}
		defer rf.Close()

		_, err = io.Copy(w, rf)
		return err
	})
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
            line-height: 1.6;
            color: #24292e;
            max-width: 980px;
            margin: 0 auto;
            padding: 45px;
            font-size: 16px;
        }
        h1, h2, h3, h4, h5, h6 {
            margin-top: 24px;
            margin-bottom: 16px;
            font-weight: 600;
            line-height: 1.25;
        }
        h1 { font-size: 2em; border-bottom: 1px solid #eaecef; padding-bottom: 0.3em; }
        h2 { font-size: 1.5em; border-bottom: 1px solid #eaecef; padding-bottom: 0.3em; }
        h3 { font-size: 1.25em; }
        h4 { font-size: 1em; }
        h5 { font-size: 0.875em; }
        h6 { font-size: 0.85em; color: #6a737d; }
        code {
            background-color: rgba(27,31,35,0.05);
            border-radius: 3px;
            font-size: 85%;
            margin: 0;
            padding: 0.2em 0.4em;
            font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
        }
        pre {
            background-color: #f6f8fa;
            border-radius: 3px;
            font-size: 85%;
            line-height: 1.45;
            overflow: auto;
            padding: 16px;
        }
        pre code {
            background-color: transparent;
            border: 0;
            display: inline;
            line-height: inherit;
            margin: 0;
            overflow: visible;
            padding: 0;
            word-wrap: normal;
        }
        table {
            border-collapse: collapse;
            border-spacing: 0;
            margin-bottom: 16px;
            width: 100%;
        }
        table th {
            font-weight: 600;
            padding: 6px 13px;
            border: 1px solid #dfe2e5;
            background-color: #f6f8fa;
        }
        table td {
            padding: 6px 13px;
            border: 1px solid #dfe2e5;
        }
        table tr {
            background-color: #fff;
            border-top: 1px solid #c6cbd1;
        }
        table tr:nth-child(2n) {
            background-color: #f6f8fa;
        }
        blockquote {
            border-left: 0.25em solid #dfe2e5;
            color: #6a737d;
            padding: 0 1em;
            margin: 0 0 16px 0;
        }
        ul, ol {
            padding-left: 2em;
            margin-bottom: 16px;
        }
        li {
            margin-bottom: 0.25em;
        }
        img {
            max-width: 100%;
            box-sizing: border-box;
        }
        hr {
            height: 0.25em;
            padding: 0;
            margin: 24px 0;
            background-color: #e1e4e8;
            border: 0;
        }
        /* Syntax highlighting - GitHub style */
        .chroma { background-color: #f6f8fa; }
        .chroma .err { color: #a61717; background-color: #e3d2d2; }
        .chroma .k { color: #d73a49; font-weight: bold; }
        .chroma .n { color: #24292e; }
        .chroma .o { color: #d73a49; font-weight: bold; }
        .chroma .cm { color: #6a737d; font-style: italic; }
        .chroma .c1 { color: #6a737d; font-style: italic; }
        .chroma .s { color: #032f62; }
        .chroma .s1 { color: #032f62; }
        .chroma .s2 { color: #032f62; }
        .chroma .mi { color: #005cc5; }
        .chroma .mf { color: #005cc5; }
        .chroma .nf { color: #6f42c1; }
        .chroma .nc { color: #6f42c1; font-weight: bold; }
        .chroma .nb { color: #005cc5; }
        .chroma .bp { color: #005cc5; }
    </style>
</head>
<body>
{{.Content}}
</body>
</html>`
