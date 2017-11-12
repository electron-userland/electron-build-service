#!/usr/bin/env bash
set -e

if [[ -f ~/.terraformrc ]] ; then
  echo "~/.terraformrc already exists.Please modify it if need manually."
  exit 1
fi

go get -u github.com/squat/terraform-provider-vultr
go get -u github.com/terraform-providers/terraform-provider-digitalocean

cat <<EOF >~/.terraformrc
providers {
  vultr = "${GOPATH:-$HOME/go}/bin/terraform-provider-vultr"
  digitalocean = "${GOPATH:-$HOME/go}/bin/terraform-provider-digitalocean"
}
EOF

rm -rf .terraform
