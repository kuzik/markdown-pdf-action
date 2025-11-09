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
	Type   string `yaml:"type"` // single | subfolders
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "render.yaml", "Path to YAML config describing render jobs")
	flag.Parse()

	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var jobs []job
	if err := yaml.Unmarshal(cfgBytes, &jobs); err != nil {
		log.Fatalf("parse yaml: %v", err)
	}

	for _, j := range jobs {
		switch j.Type {
		case "subfolders":
			if err := renderSubfolders(j); err != nil {
				log.Printf("job failed (subfolders %s): %v", j.Source, err)
			}
		case "single":
			if err := renderSingle(j); err != nil {
				log.Printf("job failed (single %s): %v", j.Source, err)
			}
		case "combine":
			if err := renderCombine(j); err != nil {
				log.Printf("job failed (combine %s): %v", j.Source, err)
			}
		default:
			log.Printf("unknown job type %q for source %s", j.Type, j.Source)
		}
	}
}

func renderSubfolders(j job) error {
	matches, err := doublestar.Glob(os.DirFS("."), j.Source)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no matches for %s", j.Source)
	}
	if err := os.MkdirAll(j.Output, 0o755); err != nil {
		return err
	}
	for _, m := range matches {
		if filepath.Base(m) == "README.md" {
			// Use parent folder name for PDF to avoid conflicts
			folder := filepath.Dir(m)
			folderName := filepath.Base(folder)
			outPDF := filepath.Join(j.Output, folderName+".pdf")

			if err := renderMarkdownToPDF(m, outPDF, folder); err != nil {
				log.Printf("render %s: %v", m, err)
			}
			if _, err := os.Stat(filepath.Join(folder, "src")); err == nil {
				zipName := filepath.Join(j.Output, folderName+"_src.zip")
				if err := zipFolder(filepath.Join(folder, "src"), zipName); err != nil {
					log.Printf("zip src %s: %v", folder, err)
				}
			}
		}
	}
	return nil
}

func renderCombine(j job) error {
	matches, err := doublestar.Glob(os.DirFS("."), j.Source)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no matches for %s", j.Source)
	}
	if err := os.MkdirAll(filepath.Dir(j.Output), 0o755); err != nil {
		return err
	}

	// Filter only README.md files and sort them
	var readmes []string
	for _, m := range matches {
		if filepath.Base(m) == "README.md" {
			readmes = append(readmes, m)
		}
	}

	if len(readmes) == 0 {
		return fmt.Errorf("no README.md files found for %s", j.Source)
	}

	// Combine all README.md files with folder names as headers
	var combined []string
	for _, m := range readmes {
		folder := filepath.Dir(m)
		folderName := filepath.Base(folder)

		// Add folder name as a header
		combined = append(combined, fmt.Sprintf("# %s\n", folderName))

		// Read and add the content
		b, err := os.ReadFile(m)
		if err != nil {
			log.Printf("warning: failed to read %s: %v", m, err)
			continue
		}
		combined = append(combined, string(b))
	}

	tmpFile, err := os.CreateTemp("", "combined-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(strings.Join(combined, "\n\n---\n\n")); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Use the first match's directory as base for resolving images
	baseDir := ""
	if len(readmes) > 0 {
		baseDir = filepath.Dir(readmes[0])
	}

	return renderMarkdownToPDF(tmpFile.Name(), j.Output, baseDir)
}

func renderSingle(j job) error {
	matches, err := doublestar.Glob(os.DirFS("."), j.Source)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no matches for %s", j.Source)
	}
	if err := os.MkdirAll(filepath.Dir(j.Output), 0o755); err != nil {
		return err
	}
	var combined []string
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			return err
		}
		combined = append(combined, string(b))
	}
	tmpFile, err := os.CreateTemp("", "combined-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(strings.Join(combined, "\n\n")); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()
	baseDir := ""
	if len(matches) > 0 {
		baseDir = filepath.Dir(matches[0])
	}
	return renderMarkdownToPDF(tmpFile.Name(), j.Output, baseDir)
}

func pdfName(md string) string {
	base := filepath.Base(md)
	return strings.TrimSuffix(base, filepath.Ext(base)) + ".pdf"
}

func renderMarkdownToPDF(mdPath, outPath, baseDir string) error {
	src, err := os.ReadFile(mdPath)
	if err != nil {
		return err
	}

	// Convert markdown to HTML with syntax highlighting
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
		return fmt.Errorf("markdown convert: %w", err)
	}

	// Determine base directory for resolving relative image paths
	if baseDir == "" {
		baseDir = filepath.Dir(mdPath)
	}

	// Embed images as base64 data URLs to avoid file:// access issues in Chrome
	htmlWithImages, err := embedImagesAsBase64(buf.String(), baseDir)
	if err != nil {
		return fmt.Errorf("embed images: %w", err)
	}

	// Wrap in HTML template with GitHub styling
	htmlContent, err := wrapHTML(htmlWithImages, filepath.Base(mdPath))
	if err != nil {
		return err
	}

	// Convert HTML to PDF using Chrome
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	return htmlToPDF(htmlContent, outPath)
}

// embedImagesAsBase64 replaces relative image paths in HTML with base64-encoded data URLs
func embedImagesAsBase64(htmlContent, baseDir string) (string, error) {
	// Pattern to match img tags with src attributes
	imgRegex := regexp.MustCompile(`<img\s+[^>]*src=["']([^"']+)["'][^>]*>`)

	result := imgRegex.ReplaceAllStringFunc(htmlContent, func(imgTag string) string {
		// Extract the src attribute value
		srcRegex := regexp.MustCompile(`src=["']([^"']+)["']`)
		matches := srcRegex.FindStringSubmatch(imgTag)
		if len(matches) < 2 {
			return imgTag
		}

		srcPath := matches[1]

		// Skip if it's already a data URL or absolute URL
		if strings.HasPrefix(srcPath, "data:") ||
			strings.HasPrefix(srcPath, "http://") ||
			strings.HasPrefix(srcPath, "https://") {
			return imgTag
		}

		// Resolve relative path
		imagePath := filepath.Join(baseDir, srcPath)

		// Read image file
		imageData, err := os.ReadFile(imagePath)
		if err != nil {
			log.Printf("Warning: failed to read image %s: %v", imagePath, err)
			return imgTag
		}

		// Determine MIME type from extension
		ext := strings.ToLower(filepath.Ext(imagePath))
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			// Default to common types
			switch ext {
			case ".png":
				mimeType = "image/png"
			case ".jpg", ".jpeg":
				mimeType = "image/jpeg"
			case ".gif":
				mimeType = "image/gif"
			case ".svg":
				mimeType = "image/svg+xml"
			case ".webp":
				mimeType = "image/webp"
			default:
				mimeType = "image/png"
			}
		}

		// Encode to base64
		base64Data := base64.StdEncoding.EncodeToString(imageData)
		dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		// Replace the src attribute with the data URL
		newImgTag := srcRegex.ReplaceAllString(imgTag, fmt.Sprintf(`src="%s"`, dataURL))

		return newImgTag
	})

	return result, nil
}

func wrapHTML(content, title string) (string, error) {
	tmpl := template.Must(template.New("page").Parse(`<!DOCTYPE html>
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
</html>`))

	var buf bytes.Buffer
	data := struct {
		Title   string
		Content template.HTML // Use template.HTML to prevent escaping HTML tags
	}{
		Title:   title,
		Content: template.HTML(content), // Convert to template.HTML so <img> tags render properly
	}
	err := tmpl.Execute(&buf, data)
	return buf.String(), err
}

func htmlToPDF(htmlContent, outPath string) error {
	// Write HTML to a temporary file
	tmpFile, err := os.CreateTemp("", "markdown-*.html")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	// Setup chrome options with file access permissions
	// --allow-file-access-from-files: Allows Chrome to load local images from file:// URLs
	// --disable-web-security: Permits cross-origin access for local files
	// These are necessary because we use <base href="file://..."> to resolve relative image paths
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.Flag("allow-file-access-from-files", true),
		chromedp.Flag("disable-web-security", true),
	)

	// Try to find Chrome/Chromium
	if chromeBin := os.Getenv("CHROME_BIN"); chromeBin != "" {
		opts = append(opts, chromedp.ExecPath(chromeBin))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var pdfBuf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate("file://"+tmpPath),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Give time for rendering
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(false).
				WithPaperWidth(8.27).   // A4 width in inches
				WithPaperHeight(11.69). // A4 height in inches
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				Do(ctx)
			return err
		}),
	); err != nil {
		return fmt.Errorf("chromedp: %w", err)
	}

	if err := os.WriteFile(outPath, pdfBuf, 0o644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}

	log.Printf("wrote %s", outPath)
	return nil
}

func zipFolder(srcDir, outZip string) error {
	f, err := os.Create(outZip)
	if err != nil {
		return err
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
