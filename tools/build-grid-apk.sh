#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
sdk=/data/data/com.termux/files/usr/opt/android-sdk
build_tools=$sdk/build-tools/35.0.0
platform=$sdk/platforms/android-35
ndk=$sdk/ndk/29.0.14206865
termux_bin=/data/data/com.termux/files/usr/bin
out=$project_dir/build/gio-kit-grid-phase2-release.apk
dest=/storage/emulated/0/Download/gio-kit-grid-phase2-release.apk
gogio_bin=$project_dir/build/gogio-termux
tool_source=$project_dir/tools/giocmd-termux

fail() {
	echo "gio-kit build: $*" >&2
	exit 1
}

[ -d "$build_tools" ] || fail "pinned build-tools 35.0.0 missing"
[ -f "$platform/android.jar" ] || fail "pinned Android platform 35 missing"
[ -d "$ndk" ] || fail "pinned NDK 29.0.14206865 missing"
[ -d "$ndk/toolchains/llvm/prebuilt/linux-x86_64" ] || fail "patched NDK host directory missing"
for tool in aapt2 d8 apksigner zipalign; do
	[ -x "$termux_bin/$tool" ] || fail "Termux-native $tool missing"
done
[ "$(readlink -f "$build_tools/aapt2")" = "$termux_bin/aapt2" ] || fail "SDK aapt2 is not the pinned Termux-native executable"
[ "$(dpkg-query -W -f='${Version}' aapt2 2>/dev/null)" = "16.0.0.4-2" ] || fail "unexpected aapt2 package version"
[ "$(go env GOARCH)" = "arm64" ] || fail "this build must run on arm64"

mkdir -p "$project_dir/build"
if [ ! -x "$gogio_bin" ] || [ "$tool_source/gogio/androidbuild.go" -nt "$gogio_bin" ]; then
	(cd "$tool_source" && GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -o "$gogio_bin" ./gogio)
fi

cd "$project_dir"
PATH="$ndk/toolchains/llvm/prebuilt/linux-x86_64/bin:$PATH"
GIO_ANDROID_PLATFORM=35 GIO_ANDROID_BUILD_TOOLS=35.0.0 ANDROID_SDK_ROOT="$sdk" ANDROID_HOME="$sdk" ANDROID_NDK_ROOT="$ndk" \
CGO_CPPFLAGS="-I$ndk/toolchains/llvm/prebuilt/linux-x86_64/sysroot/usr/include" \
CGO_CFLAGS="-Wno-error=declaration-after-statement" \
CGO_LDFLAGS="-L$ndk/toolchains/llvm/prebuilt/linux-x86_64/sysroot/usr/lib/aarch64-linux-android/24" \
"$gogio_bin" -target android -arch arm64 -appid app.giokit.cruddemo \
	-name "Gio Kit CRUD Lab" -version 0.2.0.5 -minsdk 24 -targetsdk 35 \
	-schemes gio-kit -signkey "$HOME/.android/debug.keystore" -signpass android \
	-o "$out" ./cmd/navigation-demo

native_check=$project_dir/build/libgio-navigation-check.so
unzip -p "$out" lib/arm64-v8a/libgio.so > "$native_check"
if readelf -d "$native_check" | grep -q '\[libEGL.so.1\]'; then
	fail "APK contains desktop libEGL.so.1 dependency"
fi
readelf -d "$native_check" | grep -q '\[libEGL.so\]' || fail "APK is missing Android libEGL.so dependency"
apksigner verify --verbose "$out"
cp "$out" "$dest"
[ "$(md5sum "$out" | cut -d' ' -f1)" = "$(md5sum "$dest" | cut -d' ' -f1)" ] || fail "Downloads copy checksum mismatch"

echo "$dest"
