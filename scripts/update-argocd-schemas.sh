#!/usr/bin/env bash
set -euo pipefail

version="${1:-v3.5.2}"
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
out="$root/internal/schema/data/$version"
mkdir -p "$out"
base="https://raw.githubusercontent.com/argoproj/argo-cd/$version/manifests/crds"

for pair in "application:application-crd.yaml" "applicationset:applicationset-crd.yaml"; do
  name=${pair%%:*}; file=${pair##*:}
  tmp=$(mktemp)
  trap 'rm -f "$tmp"' EXIT
  curl --fail --silent --show-error -L "$base/$file" -o "$tmp"
  ruby -ryaml -rjson -e 'doc=YAML.load_file(ARGV[0]); schema=doc.fetch("spec").fetch("versions").find{|v| v.fetch("served", true)}.fetch("schema").fetch("openAPIV3Schema"); File.write(ARGV[1], JSON.pretty_generate(schema)+"\n")' "$tmp" "$out/$name.json"
done
echo "updated schemas for $version"
