package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSelectCombineFiles_PrefersREADMEsWhenPresent(t *testing.T) {
	matches := []string{
		"lectures/02-net/README.md",
		"lectures/01-intro/README.md",
		"lectures/01-intro/notes.md",
	}

	got := selectCombineFiles(matches)
	want := []string{
		"lectures/01-intro/README.md",
		"lectures/02-net/README.md",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectCombineFiles() = %v, want %v", got, want)
	}
}

func TestSelectCombineFiles_FallsBackToAllMarkdownSorted(t *testing.T) {
	matches := []string{
		"assessment/10.md",
		"assessment/02.md",
		"assessment/01.md",
	}

	got := selectCombineFiles(matches)
	want := []string{
		"assessment/01.md",
		"assessment/02.md",
		"assessment/10.md",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectCombineFiles() = %v, want %v", got, want)
	}
}

func TestSelectCombineFiles_IgnoresNonMarkdown(t *testing.T) {
	matches := []string{
		"assessment/01.md",
		"assessment/gen_variants.py",
	}

	got := selectCombineFiles(matches)
	want := []string{"assessment/01.md"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectCombineFiles() = %v, want %v", got, want)
	}
}

func TestSectionID_README_UsesFolderName(t *testing.T) {
	if got := sectionID("lectures/01-intro/README.md"); got != "01-intro" {
		t.Fatalf("sectionID() = %q, want %q", got, "01-intro")
	}
}

func TestSectionID_FlatFile_UsesFileNameWithoutExtension(t *testing.T) {
	if got := sectionID("assessment/01.md"); got != "01" {
		t.Fatalf("sectionID() = %q, want %q", got, "01")
	}
}

func TestResolveCSS_EmptyReturnsEmpty(t *testing.T) {
	if got := resolveCSS("   "); got != "" {
		t.Fatalf("resolveCSS(blank) = %q, want empty", got)
	}
}

func TestResolveCSS_InlineIsPassedThrough(t *testing.T) {
	inline := "body { text-align: justify; }"
	if got := resolveCSS(inline); got != inline {
		t.Fatalf("resolveCSS(inline) = %q, want %q", got, inline)
	}
}

func TestResolveCSS_FilePathIsRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lectures.css")
	want := "body { hyphens: auto; }\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatalf("write temp css: %v", err)
	}

	if got := resolveCSS(path); got != want {
		t.Fatalf("resolveCSS(file) = %q, want %q", got, want)
	}
}

func TestWrapHTML_InjectsCSSAfterBaseStyles(t *testing.T) {
	marker := "body { text-align: justify; }"
	out, err := wrapHTML("<p>hi</p>", "Lecture", marker)
	if err != nil {
		t.Fatalf("wrapHTML: %v", err)
	}

	if !strings.Contains(out, marker) {
		t.Fatalf("wrapHTML output missing injected css %q", marker)
	}
	// Injected CSS must appear after the base stylesheet so it can override it.
	if strings.Index(out, marker) < strings.LastIndex(out, ".chroma") {
		t.Fatalf("injected css appears before base styles; override would fail")
	}
}

func TestWrapHTML_NoCSSOmitsExtraStyleBlock(t *testing.T) {
	out, err := wrapHTML("<p>hi</p>", "Lecture", "")
	if err != nil {
		t.Fatalf("wrapHTML: %v", err)
	}
	// Only the single base <style> block should be present.
	if got := strings.Count(out, "<style>"); got != 1 {
		t.Fatalf("<style> blocks = %d, want 1", got)
	}
}
