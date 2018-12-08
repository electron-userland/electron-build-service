## Login as Root
`sudo -i`

## Update CoreOS

```
update_engine_client -update
```

## Ubuntu

```
apt-get update && apt-get upgrade -y && curl https://releases.rancher.com/install-docker/17.03.sh | sh
apt-mark hold docker-ce
```

## Clear Rancher Node

```
curl https://gist.githubusercontent.com/develar/af014b0ac1804232f5eff5085a94c231/raw/e7ca131ab487d5962b3d135ae6b208bd6f3608a7/gistfile1.txt  | sh
```