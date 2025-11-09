package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
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
				log.Printf("Found zip for PDF %s: %s", path, zip)
			} else {
				log.Printf("No zip found for PDF %s", path)
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

		f, err := os.Create(htmlOutput)
		if err != nil {
			log.Fatalf("create: %v", err)
		}
		defer f.Close()

		tmpl := template.Must(template.New("dashboard").Parse(`<!DOCTYPE html><html><head><meta charset="utf-8"/><title>Files Dashboard</title><style>body{font-family:Arial,sans-serif;margin:20px;}table{border-collapse:collapse;width:100%;margin-bottom:30px;}th,td{border:1px solid #ddd;padding:6px;}th{background:#f4f4f4;text-align:left;}h2{margin-top:40px;border-bottom:2px solid #eee;padding-bottom:4px;}a{text-decoration:none;color:#0366d6;}a:hover{text-decoration:underline;}</style></head><body><h1>Files Dashboard</h1>{{range .}}<h2>{{.Folder}}</h2><table><thead><tr><th>File Name</th><th>Download</th><th>Source Zip</th></tr></thead><tbody>{{range .Files}}<tr><td>{{.Name}}</td><td><a href="{{.Path}}" download>Download</a></td><td>{{if .Zip}}<a href="{{.Zip}}" download>Zip</a>{{else}}-{{end}}</td></tr>{{end}}</tbody></table>{{end}}</body></html>`))
		if err := tmpl.Execute(f, ordered); err != nil {
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

		mdFile, err := os.Create(mdOutput)
		if err != nil {
			log.Fatalf("create md: %v", err)
		}
		defer mdFile.Close()

		// Build markdown template with GitHub raw URLs if available
		var mdTemplate string
		if repoURL != "" && branch != "" {
			// Get relative path from repo root to output directory
			outputDir := filepath.Dir(mdOutput)
			mdTemplate = fmt.Sprintf(`# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download](%s/%s/%s/{{.Path}}) | {{if .Zip}}[Zip](%s/%s/%s/{{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}`, repoURL, branch, outputDir, repoURL, branch, outputDir)
		} else {
			// Fallback to relative URLs
			mdTemplate = `# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download]({{.Path}}) | {{if .Zip}}[Zip]({{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}`
		}

		data := dashboardData{
			Sections:  ordered,
			RepoURL:   repoURL,
			Branch:    branch,
			OutputDir: filepath.Dir(mdOutput),
		}

		mdTmpl := template.Must(template.New("markdown").Parse(mdTemplate))
		if err := mdTmpl.Execute(mdFile, data); err != nil {
			log.Fatalf("template md: %v", err)
		}
		log.Printf("markdown dashboard written: %s", mdOutput)
	}
}
