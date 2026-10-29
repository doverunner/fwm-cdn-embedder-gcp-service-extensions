#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

mkdir -p dist
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -buildvcs=false -ldflags="-s -w" -o dist/plugin.wasm .
wasm-opt -Oz --enable-bulk-memory --enable-sign-ext --enable-nontrapping-float-to-int --enable-mutable-globals \
    dist/plugin.wasm -o dist/plugin.wasm

size="$(wc -c < dist/plugin.wasm | tr -d ' ')"
limit="$((5242880 - $(wc -c < config.json)))"
if [ "${size}" -gt "${limit}" ]; then
    echo "dist/plugin.wasm is ${size} bytes; with config.json it is over the 5 MiB plugin limit" >&2
    exit 1
fi
echo "dist/plugin.wasm: ${size} bytes"
