#!ipxe
set base-url http://releases.rancher.com/os/v1.1.0
kernel ${base-url}/vmlinuz rancher.state.dev=LABEL=RANCHER_STATE rancher.state.autoformat=[/dev/vda] rancher.state.formatzero rancher.cloud_init.datasources=[ec2]
initrd ${base-url}/initrd
boot