// Package images provides utilities for embedding images as base64 data URLs.
package images

import (
	"encoding/base64"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	imgRegex = regexp.MustCompile(`<img\s+[^>]*src=["']([^"']+)["'][^>]*>`)
	srcRegex = regexp.MustCompile(`src=["']([^"']+)["']`)
)

// EmbedImagesAsBase64 replaces relative image paths with base64 data URLs in HTML content.
func EmbedImagesAsBase64(htmlContent, baseDir string) (string, error) {
	result := imgRegex.ReplaceAllStringFunc(htmlContent, func(imgTag string) string {
		srcPath := ExtractSrcAttribute(imgTag)
		if srcPath == "" {
			return imgTag
		}

		// Skip data URLs and absolute URLs
		if IsAbsoluteOrDataURL(srcPath) {
			return imgTag
		}

		// Convert to data URL
		dataURL, err := ImageToDataURL(srcPath, baseDir)
		if err != nil {
			log.Printf("Warning: failed to embed image %s: %v", srcPath, err)
			return imgTag
		}

		return ReplaceSrcAttribute(imgTag, dataURL)
	})

	return result, nil
}

// ExtractSrcAttribute extracts the src value from an img tag.
func ExtractSrcAttribute(imgTag string) string {
	matches := srcRegex.FindStringSubmatch(imgTag)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// IsAbsoluteOrDataURL checks if a URL is absolute or a data URL.
func IsAbsoluteOrDataURL(url string) bool {
	return strings.HasPrefix(url, "data:") ||
		strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://")
}

// ImageToDataURL reads an image and converts it to a base64 data URL.
func ImageToDataURL(srcPath, baseDir string) (string, error) {
	imagePath := filepath.Join(baseDir, srcPath)

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	mimeType := GetMimeType(imagePath)
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
}

// GetMimeType determines the MIME type from file extension.
func GetMimeType(filePath string) string {
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

// ReplaceSrcAttribute replaces the src attribute in an img tag.
func ReplaceSrcAttribute(imgTag, newSrc string) string {
	return srcRegex.ReplaceAllString(imgTag, fmt.Sprintf(`src="%s"`, newSrc))
}
