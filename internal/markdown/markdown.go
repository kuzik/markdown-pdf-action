// Package markdown provides utilities for converting markdown to HTML.
package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Converter handles markdown to HTML conversion.
type Converter struct {
	md goldmark.Markdown
}

// DefaultConverter returns a converter with sensible defaults for GitHub-flavored markdown.
func DefaultConverter() *Converter {
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

	return &Converter{md: md}
}

// NewConverter creates a converter with a custom goldmark instance.
func NewConverter(md goldmark.Markdown) *Converter {
	return &Converter{md: md}
}

// ToHTML converts markdown content to HTML.
func (c *Converter) ToHTML(src []byte) (string, error) {
	var buf bytes.Buffer
	if err := c.md.Convert(src, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ToHTMLBytes converts markdown content to HTML bytes.
func (c *Converter) ToHTMLBytes(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := c.md.Convert(src, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Convert is a convenience function using the default converter.
func Convert(src []byte) (string, error) {
	return DefaultConverter().ToHTML(src)
}
