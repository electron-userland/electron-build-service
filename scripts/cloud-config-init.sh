#!/bin/sh

HOSTNAME=`wget -q -O - http://169.254.169.254/current/meta-data/hostname`

cat > "cloud-config.yaml" <<EOF
#cloud-config

hostname: $HOSTNAME
# https://github.com/number5/cloud-init/blob/master/doc/examples/cloud-config.txt
disable_root: true
ssh_authorized_keys:
  - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDY6D1wmgoedt/CYxVMcN3UxccUsWGQR0FEQMs4tjCsMK+w5JcAq9h87sNAgT7a7pRsk7r5Of/cypIAkQ39EMzQ+N5H3njR9QdN43DfTV6XclzB/qHWZdTJ9dw4xjJlXzKIHD4j695jWDtSpN/ksALwJE/EfCPzVOCL1UK21dJiFFAiiYxgt4cJy7Wpm1A2hvkZXkFmvSMgl0LXnVyZaGzxPc1BdhngpuWd66Rb/zf+plcII/eK7KUKdTdkvobzG+BxiE0N93aI4+EIRso04KT6Rx8Kmj9CrcJ6ZcCP8t5byUBDexuPbWJJKecvDigvKhOTRt9xz5uyos0gz8GK/pH7F2RTP7VqXekDGn30Oy5SYH1vVCaUqIywy6mtqjJyKR4JXZ6dLfcwf4GRFJuS0CN4UE6JUayr5m2l2GMz1WE08c/HEAM5Lv968yt311gsgWZYinTJ3E5Qllil5gSs5hndHSclBVc1Jpeb48F2kspBx71XGG0AbiX9QDoqibRc3et9D51WardAAHY4E9Awv33IqyLCbrpTFBUyNxl6RWzEH6m/cAAmgrcmp2WskPFHkH7i0lLvjSPR9iYrwbHYutXEB1sG2tbBGrGAHwRRFXzuoTETOM6vwlQ50odFhXPempr2sxdMN6KFzlSAz1hJBJLF+XtUE14lnpQqr5QWzAvfrQ== develar@gmail.com

locale: C.UTF-8
timezone: Etc/UTC

rancher:
  docker:
    engine: docker-17.06.1-ce
EOF

sudo ros install --no-reboot -f -c cloud-config.yaml -d /dev/vda
sudo reboot