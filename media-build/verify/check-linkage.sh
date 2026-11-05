#!/usr/bin/env bash
# Fails unless each binary links only glibc and needs nothing newer than
# glibc 2.28, the baseline the AlmaLinux 8 toolchain image targets.
#   check-linkage.sh <binary>...
set -euo pipefail
MAX_GLIBC=2.28
status=0
for bin in "$@"; do
  needed=$(readelf -d "$bin" | sed -n 's/.*(NEEDED).*\[\(.*\)\]/\1/p' | sort | tr '\n' ' ')
  for lib in $needed; do
    case $lib in
      libc.so.6 | libm.so.6 | libdl.so.2 | libpthread.so.0 | librt.so.1 | ld-linux-x86-64.so.2) ;;
      *) echo "$bin: links $lib, which is not part of glibc"; status=1 ;;
    esac
  done
  newest=$(objdump -T "$bin" | grep -o 'GLIBC_[0-9][0-9.]*' | sed 's/GLIBC_//' | sort -uV | tail -1)
  if [[ $(printf '%s\n%s\n' "$newest" "$MAX_GLIBC" | sort -V | tail -1) != "$MAX_GLIBC" ]]; then
    echo "$bin: needs glibc $newest, newer than $MAX_GLIBC"
    status=1
  fi
  echo "$(basename "$bin"): needs ${needed% }; glibc $newest"
done
exit "$status"
