#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
export GOMODCACHE="$PWD/.build/gomodcache"
export GOCACHE="$PWD/.build/gocache"
export GOTOOLCHAIN=local
native_go="$PWD/.build/toolchain/go/bin/go"
if [ ! -x "$native_go" ]; then native_go="$(command -v go)"; fi
export GOROOT="$(CDPATH= cd -- "$(dirname "$native_go")/.." && pwd)"
version="$($native_go env GOVERSION)"
if [ "$version" != go1.27.1 ]; then
 echo "Go 1.27.1 required; found $version; install it under .build/toolchain/go" >&2
 exit 1
fi
exec "$native_go" "$@"
