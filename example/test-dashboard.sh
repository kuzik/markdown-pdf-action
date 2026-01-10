#!/bin/bash
# Test script for files-dashboard
# Run from the repository root: ./example/test-dashboard.sh

set -e

cd "$(dirname "$0")/.."

echo "Running files-dashboard in Docker..."
docker run --rm -v "$(pwd):/github/workspace" markdown-pdf-action:local dashboard --source="example/output/" --output="example/output/index.html" --format=both

echo "Done! Check example/output/index.html and example/output/index.md for results."
