#!/usr/bin/env bash
set -e

# Vultr snapshot is not suitable for creating on demand. Minimum time to restore — 3m20s (and can be 4-5 minutes). Not acceptable. Creating snapshot also takes a lot of time.
# DigitalOcean is much better in this aspect. But we cannot use DigitalOcean since Vultr offers twice more memory. Well, creating from scratch took ~ 1m30s. It is ok.

# to see OS ids: vultr os
# to see region ids: vultr regions

# Plans:
# 201 - 5 usd (1 CPU, 1GB RAM)
# 203 - 20 usd (2 CPU, 4GB RAM)

# 9 Frankfurt
# 24 Paris
# 7 Amsterdam
# 8 London
# 3 Dallas
# OS 159 custom (to use startup script)
# OS 179 CoreOS

# 224121 - CoreOS
#vultr server create --name=electron-builder-service --hostname=electron-builder-service-green --region=9 --plan=201 --os=159 --script=223441 --user-data=scripts/private/cloud-config.yml --ipv6=true --notify-activate=false
vultr server create --name=electron-builder-service --hostname=electron-builder-service-green --region=24 --plan=201 --os=159 --script=224121 --ipv6=true --notify-activate=false
vultr server create --name=electron-builder-service --hostname=electron-builder-service-green --region=24 --plan=201 --os=159 --script=226069 --ipv6=true --notify-activate=false

# DO NOT FORGET TO CHANGE HOSTNAME

# Amsterdam
vultr server create --name=bs-ams1 --hostname=bs-ams1 --region=8 --plan=203 --os=179 --script=226072 --user-data=scripts/private/cloud-config.yml --ipv6=true --notify-activate=false

vultr server create --name=dallas1 --hostname=dallas1 --region=3 --plan=203 --os=179 --script=226072 --user-data=scripts/private/cloud-config.yml --ipv6=true --notify-activate=false