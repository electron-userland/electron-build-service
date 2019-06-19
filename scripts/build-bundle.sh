#!/bin/sh
set -ex

id=$(docker inspect --format='{{index .RepoDigests 0}}' electronuserland/build-service-builder)
echo "${id}"
sed -i "" -E "s|electronuserland/build-service-builder@[:a-z0-9]+|${id}|" k8s/builder.yaml