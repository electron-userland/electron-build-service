`brew install rancher-cli` to install [rancher](https://github.com/rancher/cli) CLI.

1. Create cluster and disable ingress controller:

    ```yaml
    ingress: 
      provider: "none"
    ```
   
   Add label `builder=true` for every node where builder should be deployed.
   
2. Import secret `tls.yaml` to create TLS secret. Or create using `Resources -> Secrets`:
    * name: `tls`,
    * data keys: `tls.cert` (expected bundle - first node cert, second certificate authority cert) and `tls.key`.

3. Import secret `papertrail-destination.yaml`. Or create using `Resources -> Secrets`:
    * name: `papertrail-destination`, 
    * key: `papertrail-destination`, 
    * value: `syslog+tls://logsN.papertrailapp.com:N?filter.name=k8s_builder_*` + (see [papertrail destination](https://papertrailapp.com/account/destinations)).
4. `make add-cluster-resources`.

## TLS

Communication encrypted using SSL. electron-builder or another client of build service uses electron-build service own CA (certificate authority) to verify connection. Bundled CA certificate and build node key/certificate intended only for local testing or own local intranet deployment.

See [How to act as a Certificate Authority](https://realtimelogic.com/blog/2014/05/How-to-act-as-a-Certificate-Authority-the-Easy-Way). Terraform is used to maintain build service CA, but project is private due to security reasons.

## Notes
  * "Import" means using Rancher `Import YAML` action.
  * If cloud provider supports BGP (e.g. Vultr), consider using [MetalLB](https://metallb.universe.tf). As cloud build service uses [OVH](https://www.ovh.ie) (no BGP for cloud VPS), [externalIPs](https://kubernetes.io/docs/concepts/services-networking/service/#external-ips) is used to expose service and not load balancer.
  * Rolling updates in Rancher differs to Kubernetes defaults — `maxUnavailable: 0` vs ` maxUnavailable: 25%`. As "absolute number is calculated from percentage by rounding down", and in our case there is the only instance per node, it doesn't matter, since for 1 pod `25%` will be `0` as absolute number and so, equals to Rancher defaults anyway. As our goal to simplify objects definitions and avoid specifying default values, `strategy.rollingUpdate` is not specified and in Rancher UI it shows as `Custom`.