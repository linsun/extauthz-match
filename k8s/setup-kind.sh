#!/usr/bin/env bash
set -euo pipefail

set -x


SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"


kind create cluster
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/standard-install.yaml

helm upgrade -i --create-namespace \
  --namespace agentgateway-system \
  --version v2.2.0-main agentgateway-crds oci://ghcr.io/kgateway-dev/charts/agentgateway-crds 

helm upgrade -i -n agentgateway-system agentgateway oci://ghcr.io/kgateway-dev/charts/agentgateway \
--version v2.2.0-main

kubectl create configmap backend-html \
  --from-file="$SCRIPT_DIR/../backend/index.html" \
  --from-file="$SCRIPT_DIR/../backend/script.js" \
  --from-file="$SCRIPT_DIR/../backend/style.css"

kubectl apply -f "$SCRIPT_DIR/allinone.yaml"

kubectl wait --for=condition=Ready gateway extauth-gateway --timeout=90s
kubectl logs deployment/extauth-server
echo kubectl port-forward deployment/extauth-gateway 8080:8080