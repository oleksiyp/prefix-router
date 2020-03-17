#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ROOT_PACKAGE="github.com/oleksiyp/prefixrouter"

# Grab code-generator version from go.sum.
CODEGEN_VERSION=$(grep 'k8s.io/code-generator' go.sum | awk '{print $2}' | head -1)
CODEGEN_PKG=$GOPATH/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}

chmod +x $CODEGEN_PKG/generate-groups.sh

$CODEGEN_PKG/generate-groups.sh all \
  "$ROOT_PACKAGE/pkg/client" \
  "$ROOT_PACKAGE/pkg/apis" \
  "prefixrouter:v1beta1" \
  --go-header-file hack/boilerplate.go.txt
