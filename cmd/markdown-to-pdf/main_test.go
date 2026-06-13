package main

import (
	"reflect"
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
