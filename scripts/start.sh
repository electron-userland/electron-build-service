#!/bin/sh
set -ex

sudo mkdir -p /etc/secrets
sudo cp /project/certs/node.crt /etc/secrets/tls.cert
sudo cp /project/certs/node.key /etc/secrets/tls.key

sudo cp /project/out/linux/builder /home/builder/builder
sudo chown builder:builder /home/builder/builder
sudo setcap CAP_NET_BIND_SERVICE=+ep /home/builder/builder
sudo --set-home --user=builder BUILDER_NODE_MODULES=/project BUILDER_HOST=`ip route get 1.2.3.4 | awk '{print $7}'` USE_EMBEDDED_ETCD=true /home/builder/builder