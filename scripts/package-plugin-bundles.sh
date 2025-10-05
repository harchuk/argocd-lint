#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <output-dir>" >&2
  exit 1
fi

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT_DIR=$1

mkdir -p "$OUT_DIR"

for bundle in "$ROOT_DIR"/bundles/*/ ; do
  name=$(basename "$bundle")
  dest="$OUT_DIR/$name.tar.gz"
  tar -czf "$dest" -C "$bundle" .
  echo "wrote $dest"
done
