#!/usr/bin/env bash
set -euo pipefail

cmd=${1:-}
shift || true

case "$cmd" in
  markdown)
    # expects --config path
    exec /usr/local/bin/markdown-to-pdf "$@"
    ;;
  dashboard)
    exec /usr/local/bin/files-dashboard "$@"
    ;;
  hydrate)
    exec /usr/local/bin/template-hydrator "$@"
    ;;
  *)
    echo "Usage: run-action.sh [markdown|dashboard|hydrate] <args>" >&2
    exit 2
    ;;
 esac
