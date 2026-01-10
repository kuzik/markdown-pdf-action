# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download]({{.RepoURL}}/{{.Branch}}/{{.Path}}) | {{if .Zip}}[Zip]({{.RepoURL}}/{{.Branch}}/{{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}
