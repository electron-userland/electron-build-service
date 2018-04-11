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