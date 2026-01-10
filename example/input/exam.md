# {{ .Title }}

**Student:** {{ .Student.Name }}  
**ID:** {{ .Student.ID }}  
**Class:** {{ .Student.Class }}  
**Date:** {{ .ExamDate }}

---

{{ range .Questions }}
## Question {{ .Number }} ({{ .Points }} points)

{{ .Text }}

---
{{ end }}
