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

type config struct {
	source string
	output string
	format string
}

// urlEncodePath encodes a file path for use in URLs
func urlEncodePath(path string) string {
	if path == "" {
		return ""
	}
	return url.PathEscape(filepath.ToSlash(path))
}

// getGitHubURL tries to get the GitHub repository URL and current branch
func getGitHubURL() (string, string) {
	repoURL := getGitRemoteURL()
	if repoURL == "" {
		return "", ""
	}

	branch := getGitBranch()
	return repoURL, branch
}

func getGitRemoteURL() string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(output))

	// Convert git@github.com:user/repo.git to https://github.com/user/repo
	if strings.HasPrefix(url, "git@github.com:") {
		url = "https://github.com/" + strings.TrimSuffix(strings.TrimPrefix(url, "git@github.com:"), ".git")
	} else if strings.HasPrefix(url, "https://github.com/") {
		url = strings.TrimSuffix(url, ".git")
	} else {
		return ""
	}

	return url
}

func getGitBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(output))
}

// buildPDFToZipMap creates a mapping of PDF files to their source zip files
func buildPDFToZipMap(source string) (map[string]string, error) {
	pdfToZip := make(map[string]string)

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Check if this is a source zip (e.g., filename_src.zip)
		if filepath.Ext(info.Name()) == ".zip" && strings.HasSuffix(info.Name(), "_src.zip") {
			baseName := strings.TrimSuffix(info.Name(), "_src.zip")
			pdfName := baseName + ".pdf"
			pdfPath := filepath.Join(filepath.Dir(path), pdfName)

			// Check if corresponding PDF exists
			if _, err := os.Stat(pdfPath); err == nil {
				rel, _ := filepath.Rel(source, path)
				pdfToZip[pdfPath] = rel
			}
		}

		return nil
	})

	return pdfToZip, err
}

// scanFiles scans the source directory and builds sections
func scanFiles(source string, pdfToZip map[string]string) ([]section, error) {
	sections := make(map[string][]fileEntry)

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip source zip files - they'll be shown in the Zip column
		if filepath.Ext(info.Name()) == ".zip" && strings.HasSuffix(info.Name(), "_src.zip") {
			return nil
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

		sections[folder] = append(sections[folder], fileEntry{
			Name: info.Name(),
			Path: rel,
			Zip:  zipRel,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return sortSections(sections), nil
}

// sortSections sorts sections and their files alphabetically
func sortSections(sections map[string][]fileEntry) []section {
	var ordered []section

	for folder, files := range sections {
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})
		ordered = append(ordered, section{Folder: folder, Files: files})
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Folder < ordered[j].Folder
	})

	return ordered
}

// adjustPathsForOutput adjusts file paths to be relative to the output file location
func adjustPathsForOutput(sections []section, source, outputDir string, urlEncode bool) []section {
	adjusted := make([]section, len(sections))

	for i, sec := range sections {
		adjustedFiles := make([]fileEntry, len(sec.Files))

		for j, file := range sec.Files {
			absSourcePath := filepath.Join(source, file.Path)
			relPath, _ := filepath.Rel(outputDir, absSourcePath)

			zipPath := ""
			if file.Zip != "" {
				absZipPath := filepath.Join(source, file.Zip)
				zipPath, _ = filepath.Rel(outputDir, absZipPath)
			}

			if urlEncode {
				relPath = urlEncodePath(relPath)
				zipPath = urlEncodePath(zipPath)
			}

			adjustedFiles[j] = fileEntry{
				Name: file.Name,
				Path: relPath,
				Zip:  zipPath,
			}
		}

		adjusted[i] = section{
			Folder: sec.Folder,
			Files:  adjustedFiles,
		}
	}

	return adjusted
}

// prepareGitHubSections prepares sections with GitHub raw URLs
func prepareGitHubSections(sections []section, source string) []section {
	githubSections := make([]section, len(sections))

	for i, sec := range sections {
		githubFiles := make([]fileEntry, len(sec.Files))

		for j, file := range sec.Files {
			zipPath := ""
			if file.Zip != "" {
				zipPath = urlEncodePath(filepath.Join(source, file.Zip))
			}

			githubFiles[j] = fileEntry{
				Name: file.Name,
				Path: urlEncodePath(filepath.Join(source, file.Path)),
				Zip:  zipPath,
			}
		}

		githubSections[i] = section{
			Folder: sec.Folder,
			Files:  githubFiles,
		}
	}

	return githubSections
}

// generateHTML creates an HTML dashboard
func generateHTML(cfg config, sections []section) error {
	htmlOutput := cfg.output
	if filepath.Ext(cfg.output) != ".html" {
		htmlOutput = strings.TrimSuffix(cfg.output, filepath.Ext(cfg.output)) + ".html"
	}

	if err := os.MkdirAll(filepath.Dir(htmlOutput), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Adjust paths for HTML output location
	htmlOutputDir := filepath.Dir(htmlOutput)
	adjustedSections := adjustPathsForOutput(sections, cfg.source, htmlOutputDir, true)

	f, err := os.Create(htmlOutput)
	if err != nil {
		return fmt.Errorf("create HTML file: %w", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("dashboard").Parse(htmlTemplate))
	if err := tmpl.Execute(f, adjustedSections); err != nil {
		return fmt.Errorf("execute HTML template: %w", err)
	}

	log.Printf("HTML dashboard written: %s", htmlOutput)
	return nil
}

// generateMarkdown creates a Markdown dashboard
func generateMarkdown(cfg config, sections []section, repoURL, branch string) error {
	mdOutput := cfg.output
	if filepath.Ext(cfg.output) != ".md" {
		mdOutput = strings.TrimSuffix(cfg.output, filepath.Ext(cfg.output)) + ".md"
	}

	if err := os.MkdirAll(filepath.Dir(mdOutput), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	outputDir := filepath.Dir(mdOutput)

	mdFile, err := os.Create(mdOutput)
	if err != nil {
		return fmt.Errorf("create markdown file: %w", err)
	}
	defer mdFile.Close()

	var tmplStr string
	var data dashboardData

	if repoURL != "" && branch != "" {
		// Use GitHub raw URLs
		githubSections := prepareGitHubSections(sections, cfg.source)
		tmplStr = fmt.Sprintf(markdownTemplateGitHub, repoURL, branch, repoURL, branch)
		data = dashboardData{
			Sections:  githubSections,
			RepoURL:   repoURL,
			Branch:    branch,
			OutputDir: outputDir,
		}
	} else {
		// Use relative URLs
		adjustedSections := adjustPathsForOutput(sections, cfg.source, outputDir, true)
		tmplStr = markdownTemplateRelative
		data = dashboardData{
			Sections:  adjustedSections,
			RepoURL:   repoURL,
			Branch:    branch,
			OutputDir: outputDir,
		}
	}

	tmpl := template.Must(template.New("markdown").Parse(tmplStr))
	if err := tmpl.Execute(mdFile, data); err != nil {
		return fmt.Errorf("execute markdown template: %w", err)
	}

	log.Printf("Markdown dashboard written: %s", mdOutput)
	return nil
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.source, "source", "output", "Directory to scan")
	flag.StringVar(&cfg.output, "output", "output/files-dashboard.html", "Dashboard output path")
	flag.StringVar(&cfg.format, "format", "both", "Output format: html, markdown, or both")
	flag.Parse()

	// Get GitHub repository information
	repoURL, branch := getGitHubURL()

	// Build mapping of PDFs to their source zips
	pdfToZip, err := buildPDFToZipMap(cfg.source)
	if err != nil {
		log.Fatalf("Failed to build PDF to ZIP mapping: %v", err)
	}

	// Scan files and build sections
	sections, err := scanFiles(cfg.source, pdfToZip)
	if err != nil {
		log.Fatalf("Failed to scan files: %v", err)
	}

	// Generate outputs based on format
	if cfg.format == "html" || cfg.format == "both" {
		if err := generateHTML(cfg, sections); err != nil {
			log.Fatalf("Failed to generate HTML: %v", err)
		}
	}

	if cfg.format == "markdown" || cfg.format == "both" {
		if err := generateMarkdown(cfg, sections, repoURL, branch); err != nil {
			log.Fatalf("Failed to generate Markdown: %v", err)
		}
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8"/>
	<title>Files Dashboard</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		table { border-collapse: collapse; width: 100%; margin-bottom: 30px; }
		th, td { border: 1px solid #ddd; padding: 6px; }
		th { background: #f4f4f4; text-align: left; }
		h2 { margin-top: 40px; border-bottom: 2px solid #eee; padding-bottom: 4px; }
		a { text-decoration: none; color: #0366d6; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<h1>Files Dashboard</h1>
	{{range .}}
	<h2>{{.Folder}}</h2>
	<table>
		<thead>
			<tr>
				<th>File Name</th>
				<th>Download</th>
				<th>Source Zip</th>
			</tr>
		</thead>
		<tbody>
			{{range .Files}}
			<tr>
				<td>{{.Name}}</td>
				<td><a href="{{.Path}}" download>Download</a></td>
				<td>{{if .Zip}}<a href="{{.Zip}}" download>Zip</a>{{else}}-{{end}}</td>
			</tr>
			{{end}}
		</tbody>
	</table>
	{{end}}
</body>
</html>`

const markdownTemplateGitHub = `# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download](%s/%s/{{.Path}}) | {{if .Zip}}[Zip](%s/%s/{{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}`

const markdownTemplateRelative = `# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download]({{.Path}}) | {{if .Zip}}[Zip]({{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}`
