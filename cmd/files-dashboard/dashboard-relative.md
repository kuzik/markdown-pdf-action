# Files Dashboard
{{range .Sections}}
## {{.Folder}}

| File Name | Download | Source Zip |
|-----------|----------|------------|
{{range .Files}}| {{.Name}} | [Download]({{.Path}}) | {{if .Zip}}[Zip]({{.Zip}}){{else}}-{{end}} |
{{end}}
{{end}}
