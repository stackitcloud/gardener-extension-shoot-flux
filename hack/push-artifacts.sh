#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

image=$(cat "$1")
chart_name=gardener-extension-shoot-flux
helm_artifacts=artifacts/charts
rm -rf "$helm_artifacts"
mkdir -p "$helm_artifacts"

function oci_repo() {
   echo "$image" | rev | cut -d'/' -f2- | rev
}

function image_repo() {
  echo "$image" | cut -d ':' -f 1
}

function image_tag() {
  echo "$(image_tag_with_digest)" | cut -d'@' -f1
}

function image_tag_with_digest(){
   echo "$image" | cut -d ':' -f 2-
}

## HELM
cp -r charts/${chart_name} "$helm_artifacts"
yq -i "\
  ( .image.repository = \"$(image_repo)\" ) | \
  ( .image.tag = \"$(image_tag_with_digest)\" )\
" "$helm_artifacts/${chart_name}/values.yaml"

# push to registry
if [ "${PUSH:-false}" != "true" ] ; then
  echo "Skip pushing artifacts because PUSH is not set to 'true'"
  exit 0
fi

helm package "$helm_artifacts/${chart_name}" --version "$(image_tag)" -d "$helm_artifacts" > /dev/null 2>&1
helm push "$helm_artifacts/${chart_name}-"* "oci://$(oci_repo)/charts"
