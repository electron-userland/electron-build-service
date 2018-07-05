## Update Portainer
```
docker pull portainer/portainer:latest
docker service update --detach=false --force portainer
```

## Login as Root
`sudo -i`

## List Docker Swarm Nodes With Labels
```
docker node ls -q | xargs docker node inspect -f '{{ .ID }} [{{ .Description.Hostname }}]: {{ range $k, $v := .Spec.Labels }}{{ $k }}={{ $v }} {{end}}'
```

## Update CoreOS

```
update_engine_client -update
```

## Ubuntu

```
apt-get update && apt-get upgrade -y && curl https://releases.rancher.com/install-docker/17.03.sh | sh
```

## Clear Rancher Node

```
curl https://gist.githubusercontent.com/develar/af014b0ac1804232f5eff5085a94c231/raw/e7ca131ab487d5962b3d135ae6b208bd6f3608a7/gistfile1.txt  | sh
```