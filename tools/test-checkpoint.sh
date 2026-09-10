#!/data/data/com.termux/files/usr/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

"$script_dir/test-guitest.sh" "$@"
CGO_ENABLED=0 "$script_dir/test-guitest.sh" "$@"
"$script_dir/test-guitest-demo.sh" "$@"
"$script_dir/test-termux.sh" "$@"
"$script_dir/generate-guitest-reference.sh" --check
