#!/bin/sh
# Regenerate references, or check them without modifying the working tree.
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
reference_dir=$(mktemp -d "${TMPDIR:-/tmp}/guitest-reference.XXXXXX")
trap 'rm -rf "$reference_dir"' EXIT HUP INT TERM
go doc -all ./guitest > "$reference_dir/api.txt"
go doc -all ./diagnostic >> "$reference_dir/api.txt"
go doc -all ./guitest/screenshot >> "$reference_dir/api.txt"
go run ./cmd/guitest help --json > "$reference_dir/commands.json"
go run ./cmd/guitest schema > "$reference_dir/schema.json"
if [ "${1:-}" = --check ]; then
	cmp "$reference_dir/api.txt" docs/guitest-api.txt
	cmp "$reference_dir/commands.json" docs/guitest-commands.json
	cmp "$reference_dir/schema.json" guitest/schema-v1.json
else
	cp "$reference_dir/api.txt" docs/guitest-api.txt
	cp "$reference_dir/commands.json" docs/guitest-commands.json
	cp "$reference_dir/schema.json" guitest/schema-v1.json
fi
