package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type fileEntry struct {
	Name string
	Path string
	Zip  string
}

type section struct {
	Folder string
	Files  []fileEntry
}

type dashboardData struct {
	Sections  []section
	RepoURL   string
	Branch    string
	OutputDir string
}

// urlEncodePath encodes a file path for use in URLs, replacing spaces and special characters
func urlEncodePath(path string) string {
	// Convert backslashes to forward slashes for consistency
	path = filepath.ToSlash(path)
	// URL encode the path
	return url.PathEscape(path)
}

// getGitHubURL tries to get the GitHub repository URL and current branch
func getGitHubURL() (string, string) {
	// Get remote URL
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	url := strings.TrimSpace(string(output))
	// Convert git@github.com:user/repo.git to https://github.com/user/repo
	if strings.HasPrefix(url, "git@github.com:") {
		url = "https://github.com/" + strings.TrimSuffix(strings.TrimPrefix(url, "git@github.com:"), ".git")
	} else if strings.HasPrefix(url, "https://github.com/") {
		url = strings.TrimSuffix(url, ".git")
	} else {
		return "", ""
	}

	// Get current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, err := cmd.Output()
	if err != nil {
		return url, "main"
	}

	return url, strings.TrimSpace(string(branchOutput))
}

func main() {
	var source string
	var output string
	var format string
	flag.StringVar(&source, "source", "output", "Directory to scan")
	flag.StringVar(&output, "output", "output/files-dashboard.html", "Dashboard output path")
	flag.StringVar(&format, "format", "both", "Output format: html, markdown, or both")
	flag.Parse()

	repoURL, branch := getGitHubURL()

	sections := map[string][]fileEntry{}

	// First pass: collect all files and build a map of PDFs to their potential source zips
	pdfToZip := map[string]string{}

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check if this is a source zip that corresponds to a PDF
		if filepath.Ext(info.Name()) == ".zip" {
			// Extract base name without _src.zip
			baseName := info.Name()
			if len(baseName) > 8 && baseName[len(baseName)-8:] == "_src.zip" {
				pdfName := baseName[:len(baseName)-8] + ".pdf"
				pdfPath := filepath.Join(filepath.Dir(path), pdfName)
				if _, err := os.Stat(pdfPath); err == nil {
					rel, _ := filepath.Rel(source, path)
					pdfToZip[pdfPath] = rel
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Fatalf("scan: %v", err)
	}

	// Second pass: build the sections with proper zip associations, excluding zip files
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip source zip files - they'll be shown in the Zip column of their PDFs
		if filepath.Ext(info.Name()) == ".zip" {
			if len(info.Name()) > 8 && info.Name()[len(info.Name())-8:] == "_src.zip" {
				return nil
			}
		}

		folder := filepath.Dir(path)
		rel, _ := filepath.Rel(source, path)

		// Check if this PDF has a corresponding source zip
		zipRel := ""
		if filepath.Ext(info.Name()) == ".pdf" {
			if zip, ok := pdfToZip[path]; ok {
				zipRel = zip
			}
		}

		sections[folder] = append(sections[folder], fileEntry{Name: info.Name(), Path: rel, Zip: zipRel})
		return nil
	})
	if err != nil {
		log.Fatalf("scan: %v", err)
	}

	// Sort folders and files
	var ordered []section
	for folder, files := range sections {
		sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
		ordered = append(ordered, section{Folder: folder, Files: files})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Folder < ordered[j].Folder })

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	// Generate HTML dashboard if requested
	if format == "html" || format == "both" {
		htmlOutput := output
		if filepath.Ext(output) != ".html" {
			htmlOutput = output[:len(output)-len(filepath.Ext(output))] + ".html"
		}

		// Adjust paths to be relative to the HTML output file location
		htmlOutputDir := filepath.Dir(htmlOutput)
		adjustedHTMLSections := make([]section, len(ordered))

		for i, sec := range ordered {
			adjustedFiles := make([]fileEntry, len(sec.Files))

			for j, file := range sec.Files {
				// Calculate path from HTML output location to source files
				absSourcePath := filepath.Join(source, file.Path)
				relPath, _ := filepath.Rel(htmlOutputDir, absSourcePath)

				zipPath := ""
				if file.Zip != "" {
					absZipPath := filepath.Join(source, file.Zip)
					zipPath, _ = filepath.Rel(htmlOutputDir, absZipPath)
				}

				adjustedFiles[j] = fileEntry{
					Name: file.Name,
					Path: urlEncodePath(relPath),
					Zip:  urlEncodePath(zipPath),
				}
			}

			adjustedHTMLSections[i] = section{
				Folder: sec.Folder,
				Files:  adjustedFiles,
			}
		}

		f, err := os.Create(htmlOutput)
		if err != nil {
			log.Fatalf("create: %v", err)
		}
		defer f.Close()

		tmpl := template.Must(template.New("dashboard").Parse(`<!DOCTYPE html><html><head><meta charset="utf-8"/><title>Files Dashboard</title><style>body{font-family:Arial,sans-serif;margin:20px;}table{border-collapse:collapse;width:100%;margin-bottom:30px;}th,td{border:1px solid #ddd;padding:6px;}th{background:#f4f4f4;text-align:left;}h2{margin-top:40px;border-bottom:2px solid #eee;padding-bottom:4px;}a{text-decoration:none;color:#0366d6;}a:hover{text-decoration:underline;}</style></head><body><h1>Files Dashboard</h1>{{range .}}<h2>{{.Folder}}</h2><table><thead><tr><th>File Name</th><th>Download</th><th>Source Zip</th></tr></thead><tbody>{{range .Files}}<tr><td>{{.Name}}</td><td><a href="{{.Path}}" download>Download</a></td><td>{{if .Zip}}<a href="{{.Zip}}" download>Zip</a>{{else}}-{{end}}</td></tr>{{end}}</tbody></table>{{end}}</body></html>`))
		if err := tmpl.Execute(f, adjustedHTMLSections); err != nil {
			log.Fatalf("template: %v", err)
		}
		log.Printf("dashboard written: %s", htmlOutput)
	}

	// Generate Markdown version if requested
	if format == "markdown" || format == "both" {
		mdOutput := output
		if filepath.Ext(output) != ".md" {
			mdOutput = output[:len(output)-len(filepath.Ext(output))] + ".md"
		}

		outputDir := filepath.Dir(mdOutput)

		// Create two sets of sections: one for GitHub URLs (with source prefix), one for relative URLs
		adjustedSections := make([]section, len(ordered))
		githubSections := make([]section, len(ordered))

		for i, sec := range ordered {
			adjustedFiles := make([]fileEntry, len(sec.Files))
			githubFiles := make([]fileEntry, len(sec.Files))

			for j, file := range sec.Files {
				// For relative URLs: path from output location to source files
				absSourcePath := filepath.Join(source, file.Path)
				relPath, _ := filepath.Rel(outputDir, absSourcePath)

				zipPath := ""
				if file.Zip != "" {
					absZipPath := filepath.Join(source, file.Zip)
					zipPath, _ = filepath.Rel(outputDir, absZipPath)
				}

				adjustedFiles[j] = fileEntry{
					Name: file.Name,
					Path: urlEncodePath(relPath),
					Zip:  urlEncodePath(zipPath),
				}

				// For GitHub URLs: prepend source path to file path and URL encode
				githubFiles[j] = fileEntry{
					Name: file.Name,
					Path: urlEncodePath(filepath.Join(source, file.Path)),
					Zip: func() string {
						if file.Zip != "" {
							return urlEncodePath(filepath.Join(source, file.Zip))
						}
						return ""
					}(),
				}
			}

			adjustedSections[i] = section{
				Folder: sec.Folder,
				Files:  adjustedFiles,
			}
			githubSections[i] = section{
				Folder: sec.Folder,
				Files:  githubFiles,
			}
		}

		mdFile, err := os.Create(mdOutput)
		if err != nil {
			log.Fatalf("create md: %v", err)
		}
		defer mdFile.Close()

		// Build markdown template with GitHub raw URLs if available
		var mdTemplate string
		var data dashboardData

		if repoURL != "" && branch != "" {
			// For GitHub URLs, use paths with source prefix
			mdTemplate = fmt.Sprintf(`# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download](%s/%s/{{.Path}}) | {{if .Zip}}[Zip](%s/%s/{{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}`, repoURL, branch, repoURL, branch)

			data = dashboardData{
				Sections:  githubSections,
				RepoURL:   repoURL,
				Branch:    branch,
				OutputDir: outputDir,
			}
		} else {
			// Fallback to relative URLs (relative to output file location)
			mdTemplate = `# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download]({{.Path}}) | {{if .Zip}}[Zip]({{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}`

			data = dashboardData{
				Sections:  adjustedSections,
				RepoURL:   repoURL,
				Branch:    branch,
				OutputDir: outputDir,
			}
		}

		mdTmpl := template.Must(template.New("markdown").Parse(mdTemplate))
		if err := mdTmpl.Execute(mdFile, data); err != nil {
			log.Fatalf("template md: %v", err)
		}
		log.Printf("markdown dashboard written: %s", mdOutput)
	}
}
