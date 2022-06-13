#!/usr/bin/env bash
set -eu

COSIGN_SECRET_NAME="cosign-public-key"
NAMESPACE="managed"
QUAY_ROBOT_ACCOUNT="hacbs-release-tests+m5_robot_account"
QUAY_SECRET_NAME="hacbs-release-tests-token"
RESOURCES_PATH="base"
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]:-$0}"; )" &> /dev/null && pwd 2> /dev/null; )";

tempDir=$(mktemp -d /tmp/m5.XXX)
trap 'rm -f "$tempDir"' EXIT

create_resources() {
  kubectl apply -k "$SCRIPT_DIR/$RESOURCES_PATH"
}

create_quay_secret() {
  podman login --username "$QUAY_ROBOT_ACCOUNT" --authfile "$tempDir/config.json" quay.io

  kubectl create secret generic "$QUAY_SECRET_NAME" -n "$NAMESPACE" \
      --from-file=.dockerconfigjson="$tempDir/config.json" --type=kubernetes.io/dockerconfigjson
}

create_cosign_secret() {
  cosign public-key --key k8s://tekton-chains/signing-secrets > "$tempDir/cosign.pub"
  kubectl create secret generic $COSIGN_SECRET_NAME -n "$NAMESPACE" --from-file="$tempDir/cosign.pub"
}

create_resources
create_quay_secret
create_cosign_secret
