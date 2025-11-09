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
    config: render.yaml
```

**Configuration File (`render.yaml`):**

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

### 2. files-dashboard

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
- Docker (optional, for image building)

### Build Locally

```bash
# Install dependencies
go mod download

# Build both commands
go build -o bin/markdown-to-pdf ./cmd/markdown-to-pdf
go build -o bin/files-dashboard ./cmd/files-dashboard
```

### Test with Example

```bash
# Render the example markdown
./bin/markdown-to-pdf --config example/render.example.yaml

# Create dashboard
./bin/files-dashboard --source example/output --output example/output/index.md --format markdown

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
# Render markdown
docker run -v $(pwd):/github/workspace markdown-pdf-action \
  markdown --config example/render.example.yaml

# Create dashboard
docker run -v $(pwd):/github/workspace markdown-pdf-action \
  dashboard --source output --output output/index.md --format markdown
```

## 📁 Repository Structure

```
.
├── cmd/
│   ├── markdown-to-pdf/     # Markdown to PDF renderer
│   │   └── main.go
│   └── files-dashboard/      # HTML dashboard generator
│       └── main.go
├── markdown-to-pdf/
│   └── action.yml            # GitHub Action definition
├── files-dashboard/
│   └── action.yml            # GitHub Action definition
├── example/
│   ├── README.md             # Example markdown with all features
│   ├── render.yaml           # Example configuration
│   └── src/                  # Example source files for zipping
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
