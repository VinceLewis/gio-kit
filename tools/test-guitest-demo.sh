#!/bin/sh
# In-process tests of the APK's root, with SQLite and no window/GPU backend.
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

dependencies=$(go list -deps -test -tags guitest ./cmd/navigation-demo)
if forbidden=$(printf '%s\n' "$dependencies" | grep -E '^gioui.org/(app|gpu)(/|$)|^gioui.org/internal/(egl|gl|vk|vulkan)(/|$)'); then
	printf 'demo test must not import window/graphics packages:\n%s\n' "$forbidden" >&2
	exit 1
fi
production_dependencies=$(go list -deps ./cmd/navigation-demo)
if forbidden=$(printf '%s\n' "$production_dependencies" | grep -E '^github.com/VinceLewis/gio-kit/guitest($|/)'); then
	printf 'production demo must not import guitest runner packages:\n%s\n' "$forbidden" >&2
	exit 1
fi
runner_dependencies=$(go list -deps -tags guitestrunner ./cmd/navigation-demo)
if forbidden=$(printf '%s\n' "$runner_dependencies" | grep -E '^gioui.org/(app|gpu)(/|$)|^gioui.org/internal/(egl|gl|vk|vulkan)(/|$)'); then
	printf 'demo script runner must not import window/graphics packages:\n%s\n' "$forbidden" >&2
	exit 1
fi
go test -tags guitest "$@" ./cmd/navigation-demo
go vet -tags guitest ./cmd/navigation-demo
go test -tags guitestrunner "$@" ./cmd/navigation-demo
go vet -tags guitestrunner ./cmd/navigation-demo
