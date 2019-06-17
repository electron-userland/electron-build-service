#!/bin/sh
set -ex

export DEBIAN_FRONTEND=noninteractive

sudo apt-get update
sudo apt-get install -qq --no-install-recommends p7zip-full authbind
sudo apt-get upgrade -qq
sudo apt-get autoremove -qq

sudo adduser --system --disabled-login --disabled-password --group builder

sudo touch /etc/authbind/byport/443
sudo chown user:user /etc/authbind/byport/443
sudo chmod 500 /etc/authbind/byport/443

sudo snap install node --classic --channel=10
sudo snap install snapcraft --classic