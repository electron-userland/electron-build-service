1. Catalog Apps -> etcd-operator from Rancher Library (not from Helm)

   Etcd Cluster Version: 3.2.23 (3.3.x not officially supported — https://github.com/coreos/etcd-operator/issues/1731).
   
2. Import `tls.yml` to create TLS secret. Or create using `Resources -> Secrets` (name `tls`, data keys: `tls.cert` and `tls.key`).

3. Import `builder.yml`. **Do not forget to update externalIPs**. See note below about MetalLB. 


## Notes
  * "Import" means using Rancher `Import YAML` action.
  * If cloud provider supports BGP (e.g. Vultr), consider using [MetalLB](https://metallb.universe.tf). As cloud build service uses [OVH](https://www.ovh.ie) (no BGP for cloud VPS), [externalIPs](https://kubernetes.io/docs/concepts/services-networking/service/#external-ips) is used to expose service and not load balancer.
  * Rolling updates in Rancher differs to Kubernetes defaults — `maxUnavailable: 0` vs ` maxUnavailable: 25%`. As "absolute number is calculated from percentage by rounding down", and in our case there is the only instance per node, it doesn't matter, since for 1 pod `25%` will be `0` as absolute number and so, equals to Rancher defaults anyway. As our goal to simplify objects definitions and avoid specifiying default values, `strategy.rollingUpdate` is not specified and in Rancher UI it shows as `Custom`.