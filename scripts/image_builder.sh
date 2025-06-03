#!/bin/bash

build=0
push=0
eva=0
NAMESPACE=${1:-kuspace}

# lets parse the arguments
# we want:
# -b for build
# --eval
# -p for push
# -q for quiet
# -h for help
# ...

usage() {
  echo "-- Deploy all manifests with kubectl --"
  echo "Usage: $0 [-b] [-p]"
  echo "  -b: also Build images"
  echo "  -p: also Push images"
  exit 1
}

parse_args() {
  while [[ "$#" -gt 0 ]]; do
    case $1 in
    -b | --build)
      build=1
      ;;
    --eval)
      eva=1
      ;;
    -p | --push)
      push=1
      ;;
    -q | --quiet)
      quiet=1
      ;;
    -h | --help)
      usage
      ;;
    --)
      shift
      break
      ;;
    -?*)
      echo "Unknown option: $1"
      usage
      ;;
    esac
    shift
  done
}

parse_args "$@"

if [ $build -eq 1 ]; then
  if [ $eva -eq 1 ]; then
    echo "eval on minikube"
    eval $(minikube docker-env)
  fi
  echo "buuuuuilding all images.............._.>"
  echo " -* minioth *- image"
  docker build --platform linux/amd64 -f build/Dockerfile.minioth -t kyri56xcaesar/kuspace:minioth-latest .
  echo " -* uspace *- image"
  docker build --platform linux/amd64 -f build/Dockerfile.uspace -t kyri56xcaesar/kuspace:uspace-latest .
  echo " -* frontapp *- image"
  docker build --platform linux/amd64 -f build/Dockerfile.frontapp -t kyri56xcaesar/kuspace:frontapp-latest .
  echo " -* wss *- image"
  docker build --platform linux/amd64 -f build/Dockerfile.wss -t kyri56xcesar/kuspace:wss-latest .

  echo " -* duckdb app *- image"
  docker build --platform linux/amd64 -f internal/uspace/applications/duckdb/Dockerfile.duck -t kyri56xcaesar/kuspace:applications-duckdb-v1 internal/uspace/applications/duckdb
  echo " -* pypandas app *- image"
  docker build --platform linux/amd64 -f internal/uspace/applications/pypandas/Dockerfile.pypandas -t kyri56xcaesar/kuspace:applications-pypandas-v1 internal/uspace/applications/pypandas
  echo " -* octave app *- image"
  docker build --platform linux/amd64 -f internal/uspace/applications/octave/Dockerfile.octave -t kyri56xcaesar/kuspace:applications-octave-v1 internal/uspace/applications/octave
  echo " -* ffmpeg app *- image"
  docker build --platform linux/amd64 -f internal/uspace/applications/ffmpeg/Dockerfile.ffmpeg -t kyri56xcaesar/kuspace:applications-ffmpeg-v1 internal/uspace/applications/ffmpeg
  echo " -* caengine app *- image"
  docker build --platform linux/amd64 -f internal/uspace/applications/caengine/Dockerfile.caengine -t kyri56xcaesar/kuspace:applications-caengine-v1 internal/uspace/applications/caengine
  echo " -* bash app *- image"
  docker build --platform linux/amd64 -f internal/uspace/applications/bash/Dockerfile.bash -t kyri56xcaesar/kuspace:applications-bash-v1 internal/uspace/applications/bash

fi

if [ $push -eq 1 ]; then
  echo "puuuuuuuuuuuuushing to the dhub"
  echo " -* minioth *- "
  docker push kyri56xcaesar/kuspace:minioth-latest
  echo " -* uspace *- "
  docker push kyri56xcaesar/kuspace:uspace-latest
  echo " -* frontapp *- "
  docker push kyri56xcaesar/kuspace:frontapp-latest
  echo " -* wss *- "
  docker push kyri56xcaesar/kuspace:wss-latest
  echo " -* duckdb app *- "
  docker push kyri56xcaesar/kuspace:applications-duckdb-v1
  echo " -* pypandas app *- "
  docker push kyri56xcaesar/kuspace:applications-pypandas-v1
  echo " -* octave app *- "
  docker push kyri56xcaesar/kuspace:applications-octave-v1
  echo " -* ffmpeg app *- "
  docker push kyri56xcaesar/kuspace:applications-ffmpeg-v1
  echo " -* caengine app *- "
  docker push kyri56xcaesar/kuspace:applications-caengine-v1
  echo " -* bash app *- "
  docker push kyri56xcaesar/kuspace:applications-bash-v1
fi
