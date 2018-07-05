#!/bin/sh
set -e

# at this moment cloud config is not applied yet, so, /etc/coreos/update.conf should be modified here
cat <<EOF >/etc/coreos/update.conf
GROUP=beta
REBOOT_STRATEGY=off
EOF

cat <<EOF >/etc/ssh/sshd_config
Subsystem sftp internal-sftp
UseDNS no
UsePAM yes
ClientAliveInterval 180
MaxAuthTries 1
PermitRootLogin no
AllowUsers core
AuthenticationMethods publickey
EOF

systemctl restart sshd.socket

mkdir -p /home/core/.ssh
chmod 600 /home/core/.ssh
echo ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDY6D1wmgoedt/CYxVMcN3UxccUsWGQR0FEQMs4tjCsMK+w5JcAq9h87sNAgT7a7pRsk7r5Of/cypIAkQ39EMzQ+N5H3njR9QdN43DfTV6XclzB/qHWZdTJ9dw4xjJlXzKIHD4j695jWDtSpN/ksALwJE/EfCPzVOCL1UK21dJiFFAiiYxgt4cJy7Wpm1A2hvkZXkFmvSMgl0LXnVyZaGzxPc1BdhngpuWd66Rb/zf+plcII/eK7KUKdTdkvobzG+BxiE0N93aI4+EIRso04KT6Rx8Kmj9CrcJ6ZcCP8t5byUBDexuPbWJJKecvDigvKhOTRt9xz5uyos0gz8GK/pH7F2RTP7VqXekDGn30Oy5SYH1vVCaUqIywy6mtqjJyKR4JXZ6dLfcwf4GRFJuS0CN4UE6JUayr5m2l2GMz1WE08c/HEAM5Lv968yt311gsgWZYinTJ3E5Qllil5gSs5hndHSclBVc1Jpeb48F2kspBx71XGG0AbiX9QDoqibRc3et9D51WardAAHY4E9Awv33IqyLCbrpTFBUyNxl6RWzEH6m/cAAmgrcmp2WskPFHkH7i0lLvjSPR9iYrwbHYutXEB1sG2tbBGrGAHwRRFXzuoTETOM6vwlQ50odFhXPempr2sxdMN6KFzlSAz1hJBJLF+XtUE14lnpQqr5QWzAvfrQ== develar@gmail.com
 > /home/core/.ssh/authorized_keys
chmod 700 /home/core/.ssh/authorized_keys

update_engine_client -update

# do not disable update service - it is convenient just log in to machine and reboot when need instead of invoke update manually
reboot