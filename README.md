# Markdown PDF Action - Lightweight Markdown to PDF Rendering

A lightweight Docker image and GitHub Actions for rendering Markdown to PDF and creating file dashboards. Built with Go and headless Chrome for high-quality, GitHub-like PDF output.

## 🎯 Goals

- **Centralized rendering logic** - Keep rendering in one place for easy maintenance
- **Code reuse** - Use across multiple projects as GitHub Actions
- **Speed up CI/CD** - Pre-built Docker images significantly faster than installing dependencies
- **Compact image size** - Optimized Docker image (~800 MB)

## 🚀 Available Actions

### 1. markdown-to-pdf

Renders Markdown files to PDF using headless Chrome with GitHub-flavored markdown support.

**Features:**
- ✅ GitHub-flavored markdown rendering
- ✅ Code blocks with syntax highlighting
- ✅ Tables with proper formatting
- ✅ Nested lists (bullets and numbered)
- ✅ Embedded images with base64 encoding
- ✅ Headings, paragraphs, blockquotes
- ✅ Task lists and text formatting
- ✅ Automatic source folder zipping
- ✅ Ukrainian and international character support

**Usage:**

```yaml
- name: Render Markdown to PDF
  uses: kuzik/markdown-pdf-action/markdown-to-pdf@v1
  with:
    config: |
      - source: "docs/**/*.md"
        output: "output/docs/"
        type: "subfolders"
      - source: "README.md"
        output: "output/README.pdf"
        type: "single"
```

**Configuration (inline YAML):**

```yaml
# Render all README.md files in subdirectories separately
- source: "docs/**/*.md"
  output: "output/docs/"
  type: "subfolders"

# Combine multiple markdown files into a single PDF
- source: "guides/*.md"
  output: "output/complete-guide.pdf"
  type: "single"

# Combine all README.md files from subfolders into one PDF
- source: "projects/**/README.md"
  output: "output/all-projects.pdf"
  type: "combine"

# Render a single file
- source: "README.md"
  output: "output/README.pdf"
  type: "single"
```

**Types:**
- `subfolders` - Renders each matched README.md file separately to the output directory, named after the parent folder. If a `src` folder exists in the same directory as the markdown file, it will be automatically zipped.
- `single` - Combines all matched files into a single PDF
- `combine` - Finds all README.md files matching the pattern and combines them into one PDF with folder names as section headers

**Custom CSS (optional `css` field):**

Each job may set a `css` field to override the built-in GitHub-like styles. The
CSS is injected *after* the base stylesheet, so it wins on any conflict. Works
with every `type`. The value is either a path to a `.css` file (relative to the
repository root) or a block of inline CSS:

```yaml
# Inline CSS — e.g. justified text for lecture handouts
- source: "lectures/**/README.md"
  output: "output/lectures/"
  type: "subfolders"
  css: |
    body { text-align: justify; hyphens: auto; }

# Or point at a stylesheet file
- source: "lectures/**/README.md"
  output: "output/lectures.pdf"
  type: "combine"
  css: "styles/lectures.css"
```

### 2. template-hydrator

Generate batches of PDFs by merging a Go template with JSON data. Perfect for creating personalized documents like exams, certificates, or reports.

**Features:**
- ✅ Go text/template syntax support
- ✅ HTML or Markdown templates
- ✅ Batch generation from JSON data
- ✅ Custom styling per document
- ✅ Automatic PDF output

**Usage:**

```yaml
- name: Hydrate Exam Templates
  uses: kuzik/markdown-pdf-action/template-hydrator@v1
  with:
    template: "templates/exam.html"
    data: "data/students.json"
    output: "dist/exams"
```

**JSON Data Structure:**

The input JSON must be a map where keys become output filenames:

```json
{
  "exam_student_001": {
    "StudentName": "John Doe",
    "Subject": "Advanced Physics",
    "Date": "2024-05-20",
    "Question1": "Explain entropy..."
  },
  "exam_student_002": {
    "StudentName": "Jane Smith",
    "Subject": "Advanced Physics",
    "Date": "2024-05-20",
    "Question1": "Discuss thermodynamics..."
  }
}
```

**Template Example:**

```html
<h1>Exam: {{ .Subject }}</h1>
<p>Student: {{ .StudentName }}</p>
<p>Date: {{ .Date }}</p>
<hr>
<div>{{ .Question1 }}</div>
```

### 3. files-dashboard

Creates an HTML dashboard with links to download all generated files.

**Features:**
- ✅ Lists all files in the output directory
- ✅ Grouped by folders
- ✅ Download links for each file
- ✅ Shows source zip files when available
- ✅ Clean, responsive HTML design

**Usage:**

```yaml
- name: Create Files Dashboard
  uses: kuzik/markdown-pdf-action/files-dashboard@v1
  with:
    source: "output/"
    output: "output/index.html"
    format: "markdown"  # Options: html, markdown, both
```

## 🛠️ Local Development

### Prerequisites

- Go 1.25 or later
- Docker (required for PDF rendering with Chrome)

### Build Locally

```bash
# Install dependencies
go mod download

# Build all commands
go build -o bin/markdown-to-pdf ./cmd/markdown-to-pdf
go build -o bin/files-dashboard ./cmd/files-dashboard
go build -o bin/template-hydrator ./cmd/template-hydrator

# Build Docker image
docker build -t markdown-pdf-action:local .
```

### Test with Example

```bash
# Run the test scripts (uses Docker)
./example/test-render.sh
./example/test-dashboard.sh
./example/test-hydrator.sh

# View results
ls -lh example/output/
```

The example includes:
- Code blocks (Python, JavaScript, Bash)
- Complex tables with GitHub styling
- Deeply nested lists
- Embedded images (logo, diagram, screenshot)
- Ukrainian text support
- Various markdown features

## 🐳 Docker Image

### Build the Image

```bash
docker build -t markdown-pdf-action .
```

The Dockerfile uses multi-stage builds:
1. **Builder stage** - Compiles Go binaries
2. **Runtime stage** - Small Debian base with Chromium and essential tools

### Run Locally

```bash
# Render markdown with inline config
docker run -v $(pwd):/github/workspace markdown-pdf-action:local \
  markdown --config='
- source: "example/input/**/*.md"
  output: "example/output/"
  type: "subfolders"
'

# Hydrate templates with data
docker run -v $(pwd):/github/workspace markdown-pdf-action:local \
  hydrate --template=templates/exam.html --data=data/students.json --output=dist/exams

# Create dashboard
docker run -v $(pwd):/github/workspace markdown-pdf-action:local \
  dashboard --source example/output --output example/output/index.html --format both
```

## 📁 Repository Structure

```
.
├── cmd/
│   ├── markdown-to-pdf/      # Markdown to PDF renderer
│   │   ├── main.go
│   │   └── template.html     # HTML template for PDF styling
│   ├── files-dashboard/      # HTML dashboard generator
│   │   ├── main.go
│   │   ├── dashboard.html    # HTML template
│   │   ├── dashboard-github.md
│   │   └── dashboard-relative.md
│   └── template-hydrator/    # Template hydration tool
│       ├── main.go
│       └── template.html     # HTML wrapper template
├── internal/                 # Shared packages
│   ├── templates/            # Template loading utilities
│   ├── markdown/             # Markdown to HTML conversion
│   ├── images/               # Image embedding (base64)
│   ├── pdf/                  # PDF generation with Chrome
│   └── ziputil/              # Zip archive utilities
├── markdown-to-pdf/
│   └── action.yml            # GitHub Action definition
├── files-dashboard/
│   └── action.yml            # GitHub Action definition
├── template-hydrator/
│   └── action.yml            # GitHub Action definition
├── example/
│   ├── input/                # Example markdown files
│   ├── output/               # Generated output
│   ├── test-render.sh        # Test script for rendering
│   └── test-dashboard.sh     # Test script for dashboard
├── Dockerfile                # Multi-stage Docker build
├── entrypoint.sh             # Action entrypoint script
├── go.mod                    # Go dependencies
└── README.md                 # This file
```

## 🔧 PDF Requirements

The PDF renderer supports:

- ✅ **Syntax highlighting** - Pygments-style formatting for code blocks
- ✅ **Images** - Relative paths from markdown file directory
- ✅ **GitHub-style rendering** - GFM (GitHub Flavored Markdown)
- ✅ **Tables** - Full table support with borders and alignment
- ✅ **Lists** - Nested lists with multiple levels
- ✅ **Typography** - Headers, bold, italic, inline code

## 📊 Dashboard Features

The HTML dashboard shows:

| Column | Description |
|--------|-------------|
| **File Name** | Name of the generated file |
| **Download** | Direct download link |
| **Source Zip** | Link to zipped source code (if applicable) |

Each folder from the source directory is displayed as a separate section.

## 📝 License

This project is designed for internal use and code reuse across multiple projects.

## 🤝 Contributing

To add features:

1. Implement in `cmd/markdown-to-pdf/main.go` or `cmd/files-dashboard/main.go`
2. Test locally with the example
3. Update documentation
4. Submit PR

## 💡 Tips

- Use glob patterns for flexible file matching: `docs/**/*.md`
- The `subfolders` type preserves directory structure
- Images should be in the same directory or subdirectory as the markdown
- Test your render config with the example before using in CI/CD

## 🐛 Troubleshooting

**PDF not generating:**
- Check YAML config syntax
- Verify glob patterns match your files
- Ensure output directory is writable

**Images not showing:**
- Images must use relative paths
- Images should be in the same directory as the markdown
- Supported formats: PNG, JPEG, GIF

**Build fails:**
- Run `go mod tidy` to update dependencies
- Ensure Go 1.25 or later is installed
- Check for compile errors in `cmd/` directories
