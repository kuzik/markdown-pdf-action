# {{ .Subject }}

**Student:** {{ .StudentName }}  
**ID:** {{ .StudentID }}  
**Date:** {{ .Date }}

---

## Question 1

{{ .Question1 }}

---

{{ if .Question2 }}
## Question 2

{{ .Question2 }}

---
{{ end }}

{{ if .Question3 }}
## Question 3

{{ .Question3 }}
{{ end }}
