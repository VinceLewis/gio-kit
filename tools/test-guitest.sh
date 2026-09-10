#!/bin/sh
# Core checks need no window, graphics libraries, SDK, or NDK.
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

dependencies=$(go list -deps -test ./guitest/... ./cmd/guitest)
if forbidden=$(printf '%s\n' "$dependencies" | grep -E '^gioui.org/(app|gpu)(/|$)|^gioui.org/internal/(egl|gl|vk|vulkan)(/|$)'); then
	printf 'guitest must not import window/graphics packages:\n%s\n' "$forbidden" >&2
	exit 1
fi
go test "$@" ./guitest/... ./cmd/guitest
go vet ./guitest/... ./cmd/guitest
