#!/bin/sh
# Adds evaluation pages from a list of "<lang> <url> [holdout]" lines: fetches
# and saves each page, checks its extracted text, writes the .txt of the pages
# that pass and drafts their labels with "?" marks. Pages marked holdout are
# added to data-text/holdout.txt; eval scores them apart and never
# prints their keywords.
#
# Usage: scripts/add-pages.sh urls.txt [-any]
set -eu

if [ $# -lt 1 ]; then
	echo "usage: $0 urls.txt [-any]" >&2
	exit 2
fi
list=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
shift

cd "$(dirname "$0")/.."
go run ./cmd/cleancheck -add "$list" "$@"
go run ./cmd/eval -draft -save=false
