#!/usr/bin/env bash
set -e

export NODE_VERSION=8.9.1

# https://github.com/hashicorp/packer/issues/2639#issuecomment-145810523
while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 2; done

sudo apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq update
sudo apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq upgrade
# nmap for ncat (logging)
sudo apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq --no-install-recommends install unzip snapcraft icnsutils rpm nmap
sudo apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq remove unattended-upgrades
sudo apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq clean
sudo apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq autoremove

# node
sudo curl -L https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-x64.tar.xz | sudo tar xJ -C /usr/local --strip-components=1
sudo unlink /usr/local/CHANGELOG.md
sudo unlink /usr/local/LICENSE
sudo unlink /usr/local/README.md
node --version

# yarn
sudo curl -L https://yarnpkg.com/latest.tar.gz | sudo tar xz -C /usr/local --strip-components=1
yarn --version

echo "Building and installing Zstd"
sudo curl -L https://github.com/electron-userland/electron-builder-binaries/releases/download/ran-0.1.3/zstd --output /usr/local/bin/zstd
sudo chmod +x /usr/local/bin/zstd

zstd --version

echo "App"

mkdir /home/app/electron-build-server
cd /home/app/electron-build-server
tar -xf /tmp/app.tar.gz -C .
yarn
yarn compile
yarn --production

# will be required for macOS
#node ./out/download-required-tools.js

# remove files not required for production
rm -rf .idea
rm -rf scripts
rm -rf src
rm -f *.iml
rm -f *.md
unlink .dockerignore
unlink .gitignore
unlink Dockerfile
unlink packer.json
unlink tsconfig.json
unlink yarn.lock

echo "Setting firewall"
sudo ufw allow OpenSSH
sudo ufw allow https
sudo ufw --force enable

# packer cannot upload because of permissions
sudo mv /tmp/electron-build-server.env /etc/electron-build-service.env
sudo mv /tmp/systemd/* /etc/systemd/system/

sudo systemctl enable /etc/systemd/system/electron-build.service