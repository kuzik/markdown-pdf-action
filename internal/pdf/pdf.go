// Package pdf provides utilities for generating PDF files using headless Chrome.
package pdf

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Options configures PDF generation settings.
type Options struct {
	// Paper dimensions in inches (default: A4)
	PaperWidth  float64
	PaperHeight float64

	// Margins in inches
	MarginTop    float64
	MarginBottom float64
	MarginLeft   float64
	MarginRight  float64

	// Print background graphics
	PrintBackground bool

	// Prefer CSS page size over paper dimensions
	PreferCSSPageSize bool

	// Timeout for Chrome operations
	Timeout time.Duration

	// Path to Chrome binary (uses default if empty)
	ChromeBin string
}

// DefaultOptions returns sensible defaults for PDF generation.
func DefaultOptions() Options {
	return Options{
		PaperWidth:        8.27,  // A4 width in inches
		PaperHeight:       11.69, // A4 height in inches
		MarginTop:         0.4,
		MarginBottom:      0.4,
		MarginLeft:        0.4,
		MarginRight:       0.4,
		PrintBackground:   true,
		PreferCSSPageSize: false,
		Timeout:           30 * time.Second,
		ChromeBin:         os.Getenv("CHROME_BIN"),
	}
}

// FromHTML converts HTML content to PDF and writes it to the output path.
func FromHTML(htmlContent, outputPath string) error {
	return FromHTMLWithOptions(htmlContent, outputPath, DefaultOptions())
}

// FromHTMLWithOptions converts HTML content to PDF with custom options.
func FromHTMLWithOptions(htmlContent, outputPath string, opts Options) error {
	// Write HTML to temporary file
	tmpFile, err := writeTempHTML(htmlContent)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// Setup Chrome context
	ctx, cancel, err := setupChromeContext(opts)
	if err != nil {
		return err
	}
	defer cancel()

	// Generate PDF
	pdfBuf, err := generatePDF(ctx, tmpFile, opts)
	if err != nil {
		return err
	}

	// Write PDF to output file
	if err := os.WriteFile(outputPath, pdfBuf, 0o644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}

	return nil
}

// writeTempHTML writes HTML content to a temporary file.
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

// setupChromeContext creates a Chrome context with appropriate options.
func setupChromeContext(opts Options) (context.Context, context.CancelFunc, error) {
	chromeOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.Flag("allow-file-access-from-files", true),
		chromedp.Flag("disable-web-security", true),
	)

	if opts.ChromeBin != "" {
		chromeOpts = append(chromeOpts, chromedp.ExecPath(opts.ChromeBin))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), chromeOpts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)

	cancel := func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}

	return ctx, cancel, nil
}

// generatePDF uses Chrome to convert HTML file to PDF.
func generatePDF(ctx context.Context, htmlPath string, opts Options) ([]byte, error) {
	var pdfBuf []byte

	err := chromedp.Run(ctx,
		chromedp.Navigate("file://"+htmlPath),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(opts.PrintBackground).
				WithPreferCSSPageSize(opts.PreferCSSPageSize).
				WithPaperWidth(opts.PaperWidth).
				WithPaperHeight(opts.PaperHeight).
				WithMarginTop(opts.MarginTop).
				WithMarginBottom(opts.MarginBottom).
				WithMarginLeft(opts.MarginLeft).
				WithMarginRight(opts.MarginRight).
				Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("chromedp: %w", err)
	}

	return pdfBuf, nil
}
