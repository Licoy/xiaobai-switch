#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
case "$(uname -s)" in Darwin) native_os=darwin;; Linux) native_os=linux;; *) echo 'Use the official Go 1.27.1 installer on Windows' >&2; exit 1;; esac
case "$(uname -m)" in arm64|aarch64) native_arch=arm64;; x86_64|amd64) native_arch=amd64;; *) echo 'Unsupported architecture' >&2; exit 1;; esac
native_filename="go1.27.1.$native_os-$native_arch.tar.gz"
mkdir -p .build
native_stage="$(mktemp -d "$PWD/.build/toolchain-download.XXXXXX")"
trap 'rm -rf "$native_stage"' EXIT HUP INT TERM
curl --fail --location --max-time 60 -o "$native_stage/downloads.json" 'https://go.dev/dl/?mode=json&include=all'
curl --fail --location --max-time 300 -o "$native_stage/$native_filename" "https://go.dev/dl/$native_filename"
python3 - "$native_stage" "$native_filename" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1]); name=sys.argv[2]
releases=json.loads((root/'downloads.json').read_text())
record=next(f for r in releases for f in r['files'] if f['filename']==name)
actual=hashlib.sha256((root/name).read_bytes()).hexdigest()
if actual!=record['sha256']:
 raise SystemExit('Official Go SHA256 mismatch')
print('Verified official SHA256:',actual)
PY
if [ -e .build/toolchain/go ]; then
 echo 'Existing .build/toolchain/go will not be replaced; remove it explicitly to reinstall' >&2
 exit 1
fi
tar -xzf "$native_stage/$native_filename" -C "$native_stage"
mkdir -p .build/toolchain
mv "$native_stage/go" .build/toolchain/go
./scripts/native-go.sh version
