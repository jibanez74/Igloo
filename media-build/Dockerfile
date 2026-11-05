# Toolchain for Igloo's Linux x64 FFmpeg build. AlmaLinux 8 gives a glibc 2.28
# runtime baseline (the same as Jellyfin's portable builds); gcc-toolset-15 and
# clang (CUDA kernels to PTX) come from AppStream. scripts/build-linux.sh does
# the actual work against sources pinned in sources.lock.
FROM almalinux:8@sha256:2d0f2eb2a83cc09dbc2eb92fade4dbe8f709628149e81aa00aec70630406603b

RUN dnf -y install dnf-plugins-core \
 && dnf config-manager --set-enabled powertools \
 && dnf -y install \
      gcc-toolset-15-gcc gcc-toolset-15-gcc-c++ gcc-toolset-15-binutils \
      gcc-toolset-15-libstdc++-devel \
      clang llvm nasm cmake make autoconf automake libtool \
      pkgconf-pkg-config git-core curl xz bzip2 patch diffutils which file \
      python3.12 python3.12-pip \
 && dnf clean all

RUN python3.12 -m pip install --no-cache-dir meson==1.12.1 ninja==1.13.2
