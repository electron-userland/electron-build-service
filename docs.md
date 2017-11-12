`brew install docker2aci`

```bash
rkt prepare --volume volume-app-certs,kind=host,/certs,readOnly=true https://github.com/electron-userland/electron-builder-binaries/releases/download/electron-build-server/electronuserland-build-server-latest.aci
```

```bash
rkt prepare https://github.com/electron-userland/electron-builder-binaries/releases/download/electron-build-server/electronuserland-build-server-latest.aci
```


https://github.com/dradtke/packer-builder-vultr
```bash
go get -d github.com/hashicorp/packer
go get -d github.com/dradtke/packer-builder-vultr
cp -r ${GOPATH:-~/go}/src/github.com/dradtke/packer-builder-vultr/vultr ${GOPATH:-~/go}/src/github.com/hashicorp/packer/builder/
```