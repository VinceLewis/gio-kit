#!/bin/sh
# Full default-package checks on the pinned Android/arm64 Termux toolchain.
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ndk=/data/data/com.termux/files/usr/opt/android-sdk/ndk/29.0.14206865
ndk_sysroot=$ndk/toolchains/llvm/prebuilt/linux-x86_64/sysroot
ndk_lib=$ndk_sysroot/usr/lib/aarch64-linux-android/24

fail() {
	printf 'gio-kit Termux tests: %s\n' "$*" >&2
	exit 1
}

cd "$project_dir"
[ "$(go env GOOS)" = android ] || fail "GOOS must be android"
[ "$(go env GOARCH)" = arm64 ] || fail "GOARCH must be arm64"
[ "$(go env CGO_ENABLED)" = 1 ] || fail "CGO_ENABLED must be 1; use tools/test-guitest.sh for pure-Go checks"
[ -d "$ndk_sysroot" ] || fail "pinned NDK 29.0.14206865 patched sysroot missing"
for header in vulkan/vulkan.h EGL/egl.h; do
	[ -r "$ndk_sysroot/usr/include/$header" ] || fail "NDK header missing: $header"
done
for library in libEGL.so liblog.so; do
	[ -r "$ndk_lib/$library" ] || fail "NDK API-24 library missing: $library"
done

# Match the APK's include/library paths and existing NDK C-warning workaround.
# Standalone CGO test executables additionally need Android's logging library.
# Override CGO flags only for this script and its children, never with go env -w.
export CGO_CPPFLAGS="-I$ndk_sysroot/usr/include"
export CGO_CFLAGS="-Wno-error=declaration-after-statement"
export CGO_LDFLAGS="-L$ndk_lib -llog"

# Optional arguments are go test flags (for example, -count=1), not vet flags.
# The separate test-guitest scripts check GPU isolation and tagged demo tests.
go test "$@" ./...
go vet ./...
