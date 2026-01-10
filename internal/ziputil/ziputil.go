// Package ziputil provides utilities for creating zip archives.
package ziputil

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CreateFromFolder creates a zip archive of a directory.
func CreateFromFolder(srcDir, outZip string) error {
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

// CreateFromFiles creates a zip archive from a list of files.
// baseDir is used to compute relative paths within the archive.
func CreateFromFiles(files []string, baseDir, outZip string) error {
	f, err := os.Create(outZip)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for _, file := range files {
		if err := addFileToZip(zw, file, baseDir); err != nil {
			return err
		}
	}

	return nil
}

// addFileToZip adds a single file to a zip writer.
func addFileToZip(zw *zip.Writer, filePath, baseDir string) error {
	rel, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		rel = filepath.Base(filePath)
	}

	w, err := zw.Create(rel)
	if err != nil {
		return err
	}

	rf, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer rf.Close()

	_, err = io.Copy(w, rf)
	return err
}
