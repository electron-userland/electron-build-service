#!/usr/bin/env bash
set -e

# at this moment cloud config is not applied yet, so, /etc/coreos/update.conf should be modified here
cat <<EOF >/etc/coreos/update.conf
GROUP=beta
REBOOT_STRATEGY=off
EOF

update_engine_client -update

systemctl enable /etc/systemd/system/papertrail.service
systemctl mask fleet.socket --now

# 3600 - 24 hour
ipset create blocked_hosts hash:ip timeout 86400
iptables -I INPUT 1 -m set -j DROP  --match-set blocked_hosts src
iptables -I FORWARD 1 -m set -j DROP  --match-set blocked_hosts src
# ipset add blocked_hosts 13.93.161.130

reboot