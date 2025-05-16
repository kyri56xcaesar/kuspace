#!/bin/bash
set -e

echo "🔧 Building all images"
echo "Building minioth image"
docker buildx build --platform linux/amd64 -f builds/Dockerfile.minioth -t kyri56xcaesar/kuspace:minioth-latest . --push
echo "Building uspace image"
docker buildx build --platform linux/amd64 -f builds/Dockerfile.uspace -t kyri56xcaesar/kuspace:uspace-latest . --push
echo "Building frontapp image"
docker buildx build --platform linux/amd64 -f builds/Dockerfile.frontapp -t kyri56xcaesar/kuspace:frontapp-latest . --push
echo "Building duckdb app image"
docker buildx build --platform linux/amd64 -f internal/uspace/applications/duckdb/Dockerfile.duck -t kyri56xcaesar/kuspace:applications/duckdb-latest internal/uspace/applications/duckdb --push

NAMESPACE=${1:-kuspace}
export NAMESPACE

echo "🔧 Creating namespace $NAMESPACE"
kubectl create namespace "$NAMESPACE" 2>/dev/null || true

echo "🚀 Applying all manifests from deployments/ with NAMESPACE=$NAMESPACE"
find deployments -type f -name '*.yaml' | while read -r file; do
  echo "Applying $file"
  envsubst < "$file" | kubectl apply -f -
done



