#!/bin/bash
# Test script for markdown-to-pdf with inline config
# Run from the repository root: ./example/test-render.sh

set -e

cd "$(dirname "$0")/.."

CONFIG='
- source: "example/input/dir/**/*.md"
  output: "example/output/markdown-pdfs/"
  type: "subfolders"

- source: "example/input/docs/*.md"
  output: "example/output/docs.pdf"
  type: "single"

- source: "example/input/README.md"
  output: "example/output/README.pdf"
  type: "single"

- source: "example/input/dir/**/*.md"
  output: "example/output/combined-projects.pdf"
  type: "combine"
'

echo "Running markdown-to-pdf in Docker..."
docker run --rm -v "$(pwd):/github/workspace" markdown-pdf-action:local markdown --config="$CONFIG"

echo "Done! Check example/output/ for results."
