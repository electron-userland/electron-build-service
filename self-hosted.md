To deploy, [kubernetes](https://kubernetes.io/) is used for security reasons. Linux is required, but you can use [multipass](https://github.com/CanonicalLtd/multipass) to run on macOS/Windows.

Two steps:

1. create kubernetes cluster,
2. deploy service.

## Kubernetes Cluster Creation

[microk8s](https://microk8s.io) is highly recommended and used in this instruction.

Please see script [install-local-k8s.sh](scripts/install-local-k8s.sh) and execute it (one-line command to copy and paste is not provided to ensure that you will not damage your machine if script will be maliciously replaced).

## Deploy Build Service

```shell script

# to ensure that microk8s is ready
microk8s.status --wait-ready
# service will be deployed to namespace `build-service` 
curl -fsSL https://raw.githubusercontent.com/electron-userland/electron-build-service/master/k8s/generated/self-hosted.yaml | microk8s.kubectl --namespace=build-service apply -f - 

```

## Client Configuration

To configure client (e.g. electron-builder) to use your own build service, please set following environment variables:

```
BUILD_AGENT_HOST=SERVER_IP:30443 USE_BUILD_SERVICE_LOCAL_CA=true
```

replace `SERVER_IP` to IP address of you server. If you use [multipass](https://github.com/CanonicalLtd/multipass), execute `multipass info build-service` to know `IPv4` address.

Here `USE_BUILD_SERVICE_LOCAL_CA=true` is used, because configuring own certificate authority is not required for your own internal installation.

To force remote build, env `_REMOTE_BUILD=true` can be used.

## Cluster Management

To manage k8s cluster, consider using [Kubernetic](https://kubernetic.com/), because it is not possible to use [k8s dashboard](https://github.com/kubernetes/dashboard/issues/2735#issuecomment-355548346) outside of virtual machine.

Please note that service is deployed to custom `build-service` namespace (not to default).

### macOS
```shell script
# https://kubernetic.com/
# install kubectl if not yeat installed: brew install kubectl
multipass exec build-service -- /snap/bin/microk8s.config > ~/.kube/config
```

### Linux

```shell script
/snap/bin/microk8s.config > ~/.kube/config
```

## Multipass

[Multipass](https://github.com/CanonicalLtd/multipass) orchestrates virtual Ubuntu instances. Because to run k8s cluster you need Linux, so, on macOS/Windows you need a virtual machine.


To create:

```shell script
multipass launch --name build-service --cpus 4 18.04
```

And then use `multipass shell build-service` to execute commands in the virtual machine.

## Cheat Sheet

* To add alias (use `kubectl` directly instead of `microk8s.kubectl`): `sudo snap alias microk8s.kubectl kubectl`
* To expose dashboard (doesn't work if inside VM because cannot be accessed externally):
    ```shell script
    # to open k8s dashboard (see IP in multipass info build-service): 
    # http://192.168.64.4:8001/api/v1/namespaces/kube-system/services/https:kubernetes-dashboard:/proxy/
    microk8s.kubectl proxy --accept-hosts=.* --address=0.0.0.0 &
    ```