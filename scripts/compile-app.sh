#!/usr/bin/env bash
set -e

yarn compile
yarn --production --frozen-lockfile

rm -rf src
rm -rf scripts
unlink yarn.lock
unlink .yarnclean
unlink tsconfig.json
unlink Dockerfile