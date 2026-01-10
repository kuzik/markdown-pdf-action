#!/bin/bash
# Test script for template-hydrator
# Run from the repository root: ./example/test-hydrator.sh

set -e

cd "$(dirname "$0")/.."

echo "=== Test 1: Simple HTML template (wrapped with base styles) ==="
docker run --rm -v "$(pwd):/github/workspace" markdown-pdf-action:local \
  hydrate \
  --template="example/input/exam.html" \
  --data="example/input/exams.json" \
  --output="example/output/exams"

echo ""
echo "=== Test 2: Complete HTML template with custom styles (used directly) ==="
docker run --rm -v "$(pwd):/github/workspace" markdown-pdf-action:local \
  hydrate \
  --template="example/input/exam-styled.html" \
  --data="example/input/exams.json" \
  --output="example/output/exams-styled"

echo ""
echo "=== Test 3: Markdown template (wrapped with base styles) ==="
docker run --rm -v "$(pwd):/github/workspace" markdown-pdf-action:local \
  hydrate \
  --template="example/input/exam.md" \
  --data="example/input/exams.json" \
  --output="example/output/exams-md"

echo ""
echo "Done! Check example/output/ for results:"
echo ""
echo "exams/        - Simple HTML templates wrapped with base styles"
echo "exams-styled/ - Custom styled HTML templates (no wrapping)"
echo "exams-md/     - Markdown templates converted and wrapped"
echo ""
ls -la example/output/exams/ example/output/exams-styled/ example/output/exams-md/ 2>/dev/null || true
