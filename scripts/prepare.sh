#!/usr/bin/env bash
set -e

# https://github.com/hashicorp/packer/issues/2639#issuecomment-145810523
while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 2; done