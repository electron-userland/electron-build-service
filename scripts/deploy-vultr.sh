#!/usr/bin/env bash
set -e

# Vultr snapshot is not suitable for creating on demand. Minimum time to restore — 3m20s (and can be 4-5 minutes). Not acceptable. Creating snapshot also takes a lot of time.
# DigitalOcean is much better in this aspect. But we cannot use DigitalOcean since Vultr offers twice more memory. Well, creating from scratch took ~ 1m30s. It is ok.

# to see OS ids: vultr os
# to see region ids: vultr regions

# 9 Frankfurt
# OS 159 custom (to use startup script)
vultr server create --name=electron-builder-service --hostname=electron-builder-service-green --region=9 --plan=201 --os=159 --script=223441 --user-data=scripts/private/cloud-config.yml --ipv6=true --notify-activate=false