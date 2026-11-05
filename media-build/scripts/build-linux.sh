#!/usr/bin/env bash
# Runs inside the media-build container (see ../Dockerfile and ../build.sh):
# fetches and verifies the sources in ../sources.lock, builds the static
# dependencies, applies ../patches to FFmpeg, and writes out/linux_amd64/.
set -euo pipefail
source /opt/rh/gcc-toolset-15/enable

B=/build
CACHE=$B/cache
WORK=$B/work
SRC=$WORK/src
PREFIX=$WORK/prefix
STAMPS=$WORK/stamps
OUT=$B/out/linux_amd64
JOBS=${JOBS:-$(nproc)}
mkdir -p "$HOME" "$CACHE" "$SRC" "$PREFIX/lib" "$STAMPS" "$OUT"
# zimg.pc asks for -lstdc++; with the archive first on the -L path that
# resolves statically, so the binaries need no libstdc++.so or libgcc_s.
ln -sf "$(g++ -print-file-name=libstdc++.a)" "$PREFIX/lib/libstdc++.a"

export PKG_CONFIG_LIBDIR=$PREFIX/lib/pkgconfig:$PREFIX/share/pkgconfig
export CFLAGS="-I$PREFIX/include -O2 -pipe -fPIC -DPIC -D_FORTIFY_SOURCE=2 -fstack-protector-strong -pthread"
export CXXFLAGS=$CFLAGS
export LDFLAGS="-L$PREFIX/lib -static-libgcc -static-libstdc++ -Wl,-z,relro,-z,now -pthread"

die() { echo "build-linux: $*" >&2; exit 1; }
log() { echo "==> $*"; }

# lock_field <name> <column>: 2=kind 3=ref 4=sha256 5=url
lock_field() {
  awk -v n="$1" -v c="$2" '$1 == n { print $c; found = 1 } END { exit !found }' "$B/sources.lock" ||
    die "$1 is not in sources.lock"
}

# fetch <name>: makes sure $CACHE holds the verified source tarball and prints its path.
fetch() {
  local name=$1 kind ref sha url tarball tmp
  kind=$(lock_field "$name" 2)
  ref=$(lock_field "$name" 3)
  sha=$(lock_field "$name" 4)
  url=$(lock_field "$name" 5)
  if [[ $kind == tar ]]; then
    tarball=$CACHE/$(basename "$url")
    if [[ ! -f $tarball ]]; then
      curl -fsSL --retry 3 -o "$tarball.part" "$url" >&2
      mv "$tarball.part" "$tarball"
    fi
    echo "$sha  $tarball" | sha256sum -c --quiet - >&2 || die "$name: sha256 mismatch for $tarball"
  else
    # Git sources are pinned by commit. The tree, submodules included, is
    # packed once into a normalized tarball that also goes into the
    # corresponding-source bundle.
    tarball=$CACHE/$name-$ref.tar.gz
    if [[ ! -f $tarball ]]; then
      tmp=$(mktemp -d "$WORK/fetch-$name.XXXXXX")
      git -C "$tmp" init -q
      git -C "$tmp" fetch -q --depth 1 "$url" "$ref"
      git -C "$tmp" -c advice.detachedHead=false checkout -q FETCH_HEAD
      [[ $(git -C "$tmp" rev-parse HEAD) == "$ref" ]] || die "$name: fetched commit is not $ref"
      git -C "$tmp" submodule -q update --init --recursive --depth 1
      tar -C "$tmp" --exclude=.git --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner \
        --transform "s,^\.,$name-$ref," -cf - . | gzip -n -9 > "$tarball.part"
      mv "$tarball.part" "$tarball"
      rm -rf "$tmp"
    fi
  fi
  echo "$tarball"
}

# unpack <name>: fresh source tree at $SRC/<name>.
unpack() {
  local tarball
  tarball=$(fetch "$1")
  rm -rf "${SRC:?}/$1"
  mkdir -p "$SRC/$1"
  tar -xf "$tarball" -C "$SRC/$1" --strip-components=1
}

# built <name>: true when <name> was already built from its current ref.
built() { [[ -f $STAMPS/$1 && $(cat "$STAMPS/$1") == "$(lock_field "$1" 3)" ]]; }
mark_built() { lock_field "$1" 3 > "$STAMPS/$1"; }

# gen_implib <lib.so> <lib.a>: a static stub that dlopens the host's library on
# first use, so ffmpeg starts on hosts without it and uses the host's VA-API
# stack (and drivers) when present. Same approach as Jellyfin/BtbN builds.
gen_implib() {
  local tmp
  tmp=$(mktemp -d "$WORK/implib.XXXXXX")
  (
    cd "$tmp"
    python3.12 "$SRC/implib/implib-gen.py" --target x86_64-linux-gnu --dlopen --lazy-load "$1" > /dev/null
    gcc $CFLAGS -fvisibility=hidden -Wa,--noexecstack -DIMPLIB_HIDDEN_SHIMS -c ./*.tramp.S ./*.init.c
    ar -rcs "$2" ./*.tramp.o ./*.init.o
  )
  rm -rf "$tmp"
}

build_zlib() {
  cd "$SRC/zlib"
  ./configure --prefix="$PREFIX" --static
  make -j"$JOBS"
  make install
}

build_x264() {
  cd "$SRC/x264"
  ./configure --prefix="$PREFIX" --disable-cli --enable-static --enable-pic --disable-lavf --disable-swscale
  make -j"$JOBS"
  make install
}

build_zimg() {
  cd "$SRC/zimg"
  ./autogen.sh
  ./configure --prefix="$PREFIX" --disable-shared --enable-static --with-pic
  make -j"$JOBS"
  make install
}

build_dav1d() {
  cd "$SRC/dav1d"
  meson setup build --prefix="$PREFIX" --libdir=lib --buildtype=release --default-library=static \
    -Denable_tools=false -Denable_tests=false
  ninja -C build -j"$JOBS"
  ninja -C build install
}

build_ffnvcodec() {
  cd "$SRC/ffnvcodec"
  make PREFIX="$PREFIX" install
}

build_libvpl() {
  cd "$SRC/libvpl"
  cmake -GNinja -S . -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX="$PREFIX" \
    -DCMAKE_INSTALL_LIBDIR=lib -DINSTALL_DEV=ON -DINSTALL_LIB=ON -DINSTALL_EXAMPLES=OFF \
    -DBUILD_EXAMPLES=OFF -DBUILD_EXPERIMENTAL=OFF -DBUILD_SHARED_LIBS=OFF -DBUILD_TESTS=OFF
  ninja -C build -j"$JOBS"
  ninja -C build install
}

# libdrm and libva are built shared only to generate the implib stubs above.
build_libdrm() {
  cd "$SRC/libdrm"
  meson setup build --prefix="$PREFIX" --libdir=lib --buildtype=release -Ddefault_library=shared \
    -Dudev=false -Dtests=false -Dman-pages=disabled -Dcairo-tests=disabled -Dvalgrind=disabled \
    -Dintel=disabled -Dradeon=disabled -Damdgpu=disabled -Dnouveau=disabled -Dvmwgfx=disabled \
    -Domap=disabled -Dexynos=disabled -Dfreedreno=disabled -Dtegra=disabled -Dvc4=disabled \
    -Detnaviv=disabled
  ninja -C build -j"$JOBS"
  ninja -C build install
}

build_libva() {
  cd "$SRC/libva"
  meson setup build --prefix="$PREFIX" --libdir=lib --buildtype=release -Ddefault_library=shared \
    -Denable_docs=false -Ddisable_drm=false -Dwith_x11=no -Dwith_glx=no -Dwith_wayland=no
  ninja -C build -j"$JOBS"
  ninja -C build install
  gen_implib "$PREFIX/lib/libdrm.so.2" "$PREFIX/lib/libdrm.a"
  gen_implib "$PREFIX/lib/libva.so.2" "$PREFIX/lib/libva.a"
  gen_implib "$PREFIX/lib/libva-drm.so.2" "$PREFIX/lib/libva-drm.a"
  rm -f "$PREFIX"/lib/libdrm*.so* "$PREFIX"/lib/libva*.so*
}

# --disable-autodetect makes every external feature explicit, so the build
# never depends on what the image happens to contain. It also turns off
# threads, hence --enable-pthreads.
FFMPEG_FEATURES=(
  --enable-gpl
  --disable-autodetect
  --disable-shared --enable-static
  --disable-debug --disable-doc --disable-ffplay --disable-network
  --enable-pthreads --enable-iconv --enable-zlib
  --enable-libx264 --enable-libzimg --enable-libdav1d
  --enable-ffnvcodec --enable-cuda --enable-cuda-llvm --enable-cuvid --enable-nvdec --enable-nvenc
  --enable-libvpl --enable-vaapi --enable-libdrm
)

build_ffmpeg() {
  local patch
  cd "$SRC/ffmpeg"
  for patch in "$B"/patches/*.patch; do
    log "applying $(basename "$patch")"
    patch -p1 --forward --fuzz=0 --no-backup-if-mismatch -s < "$patch"
  done
  # g++ links the executables so -static-libstdc++ applies to zimg and libvpl.
  ./configure --prefix="$WORK/ffmpeg-prefix" \
    --pkg-config=pkg-config --pkg-config-flags=--static \
    --cc=gcc --cxx=g++ --ld=g++ --nvcc=clang \
    --extra-version=Igloo \
    --extra-cflags="$CFLAGS" --extra-cxxflags="$CXXFLAGS" --extra-ldflags="$LDFLAGS" \
    --extra-ldexeflags=-pie --extra-libs=-ldl \
    "${FFMPEG_FEATURES[@]}"
  make -j"$JOBS"
}

for dep in implib zlib x264 zimg dav1d ffnvcodec libvpl libdrm libva; do
  if built "$dep"; then
    log "$dep: up to date"
    continue
  fi
  log "$dep: building"
  unpack "$dep"
  [[ $dep == implib ]] || "build_$dep"
  mark_built "$dep"
done

log "ffmpeg: building"
unpack ffmpeg
build_ffmpeg

log "packaging"
install -m 0755 "$SRC/ffmpeg/ffmpeg" "$SRC/ffmpeg/ffprobe" "$OUT/"
(cd "$OUT" && sha256sum ffmpeg ffprobe > SHA256SUMS)

{
  echo "Igloo FFmpeg build, linux_amd64"
  echo
  "$OUT/ffmpeg" -hide_banner -version | head -1
  echo
  echo "## Toolchain"
  echo "base image: $(grep -m1 '^FROM ' "$B/Dockerfile" | cut -d' ' -f2)"
  gcc --version | head -1
  clang --version | head -1
  rpm -q gcc-toolset-15-gcc gcc-toolset-15-binutils clang nasm cmake
  echo "meson $(meson --version), ninja $(ninja --version)"
  echo
  echo "## Configure"
  grep -m1 '^FFMPEG_CONFIGURATION=' "$SRC/ffmpeg/ffbuild/config.mak" | cut -d= -f2-
  echo
  echo "## Sources (sources.lock)"
  grep -v '^#' "$B/sources.lock"
  echo
  echo "## Patches"
  (cd "$B/patches" && sha256sum ./*.patch)
  echo
  echo "## Binaries"
  cat "$OUT/SHA256SUMS"
} > "$OUT/BUILDINFO"

# Corresponding source for the GPL: every source tarball, the patches and the
# recipe that produced these binaries.
version=$(lock_field ffmpeg 3)
bundle=$B/out/igloo-ffmpeg-$version-sources.tar
sources=()
for name in $(awk '!/^#/ && NF { print $1 }' "$B/sources.lock"); do
  sources+=("cache/$(basename "$(fetch "$name")")")
done
tar -C "$B" --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner -cf "$bundle" \
  Dockerfile build.sh sources.lock scripts patches "${sources[@]}"

log "done: $OUT"
ls -l "$OUT" "$bundle"
