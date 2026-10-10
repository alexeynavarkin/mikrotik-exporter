#!/usr/bin/env bash
# Download the layers (amd64 or $ARCH) of a Docker Hub image via the registry API, no docker daemon.
# Usage: pull-layers.sh <repo> <tag> <dest-dir>   (env ARCH=amd64|arm64)
set -euo pipefail
REPO=$1 TAG=$2 DEST=$3 ARCH=${ARCH:-amd64}
mkdir -p "$DEST"
REG=https://registry-1.docker.io/v2/$REPO
TOK=$(curl -fsS "https://auth.docker.io/token?service=registry.docker.io&scope=repository:$REPO:pull" |
      python3 -I -c 'import json,sys;print(json.load(sys.stdin)["token"])')
ACC='application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json'
hdr=(-H "Authorization: Bearer $TOK" -H "Accept: $ACC")
idx=$(curl -fsS "${hdr[@]}" "$REG/manifests/$TAG")
dig=$(printf '%s' "$idx" | ARCH=$ARCH python3 -I -c '
import json,sys,os
d=json.load(sys.stdin)
print([m["digest"] for m in d.get("manifests",[]) if m.get("platform",{}).get("architecture")==os.environ["ARCH"]][0] if "manifests" in d else "")')
if [ -n "$dig" ]; then man=$(curl -fsS "${hdr[@]}" "$REG/manifests/$dig"); else man=$idx; fi
printf '%s' "$man" > "$DEST/manifest.json"
cfg=$(printf '%s' "$man" | python3 -I -c 'import json,sys;print(json.load(sys.stdin)["config"]["digest"])')
curl -fsSL "${hdr[@]}" -o "$DEST/config.json" "$REG/blobs/$cfg"
for l in $(printf '%s' "$man" | python3 -I -c 'import json,sys;[print(x["digest"]) for x in json.load(sys.stdin)["layers"]]'); do
  f="$DEST/${l#sha256:}.tar"
  [ -s "$f" ] || curl -fsSL "${hdr[@]}" -o "$f" "$REG/blobs/$l"
  [ "$(sha256sum "$f" | cut -d' ' -f1)" = "${l#sha256:}" ] || { echo "digest mismatch $f" >&2; exit 1; }
  echo "$f"
done
