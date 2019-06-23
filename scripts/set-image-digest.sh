#!/bin/sh
set -ex

# kustomize set image doesn't work (well, it doesn't have proper docs and not clear how to make it works)

id=$(docker inspect --format='{{index .RepoDigests 0}}' electronuserland/build-service-builder)
sed -i "" -E "s|electronuserland/build-service-builder@[:a-z0-9]+|${id}|" k8s/base/builder.yaml