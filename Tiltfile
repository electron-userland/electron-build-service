k8s_resource("builder", trigger_mode=TRIGGER_MODE_MANUAL)

k8s_yaml(kustomize('k8s/overlays/single-node'))

docker_build("electronuserland/build-service-builder", ".", dockerfile="cmd/builder/Dockerfile")