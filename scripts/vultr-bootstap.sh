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
# do not disable update service - it is convenient just log in to machine and reboot when need instead of invoke update manually

reboot