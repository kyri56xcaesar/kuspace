#!/bin/bash
set -e

NAMESPACE=${1:-kuspace}

echo " Deleting all resources in namespace $NAMESPACE"
kubectl delete namespace "$NAMESPACE" --ignore-not-found
