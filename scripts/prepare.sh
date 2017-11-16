#!/usr/bin/env bash
set -e

# https://github.com/hashicorp/packer/issues/2639#issuecomment-145810523
while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 2; done

sudo mv /tmp/rc.local /etc/rc.local
sudo chown root:root /etc/rc.local
sudo chmod 755 /etc/rc.local

sudo mv /tmp/rc.local /etc/sshd_config
sudo chown root:root /etc/sshd_config
sudo chmod 600 /etc/sshd_config

wait-for-docker

docker pull electronuserland/build-server