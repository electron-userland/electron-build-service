#!/bin/sh
mount | grep /dev/vda >/dev/null
RETVAL=$?
if [ $RETVAL -eq 0 ]; then
  exit 0
fi
sudo dd if=/dev/zero of=/dev/vda bs=1M count=1
logger -t start.sh "Prepared /dev/vda for use as Rancher state disk. Rebooting."
sudo reboot