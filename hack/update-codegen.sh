#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ROOT_PACKAGE="github.com/oleksiyp/prefix-router"

SCRIPT_ROOT=$(git rev-parse --show-toplevel)

# Grab code-generator version from go.sum.
CODEGEN_VERSION=$(grep 'k8s.io/code-generator' go.sum | awk '{print $2}' | head -1)
CODEGEN_PKG=$(echo `go env GOPATH`"/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}")

cd $CODEGEN_PKG

chmod +x ./generate-groups.sh

./generate-groups.sh all \
  "$ROOT_PACKAGE/pkg/client" \
  "$ROOT_PACKAGE/pkg/apis" \
  "prefix-router:v1beta1"
