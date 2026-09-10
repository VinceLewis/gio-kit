#!/bin/sh
# Optional renderer probe, isolated from the graphics-free core suite.
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
test_sysroot=/data/data/com.termux/files/usr/opt/android-sdk/ndk/29.0.14206865/toolchains/llvm/prebuilt/linux-x86_64/sysroot
[ -r "$test_sysroot/usr/include/EGL/egl.h" ]
[ "$(go env CGO_ENABLED)" = 1 ]
export CGO_CPPFLAGS="-I$test_sysroot/usr/include"
export CGO_CFLAGS="-Wno-error=declaration-after-statement"
export CGO_LDFLAGS="-L$test_sysroot/usr/lib/aarch64-linux-android/24 -llog"
go test "$@" -tags guitestgpu ./guitest/screenshot
go vet -tags guitestgpu ./guitest/screenshot
