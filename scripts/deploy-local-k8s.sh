#!/bin/sh -ex

# microk8s.reset

microk8s.status --wait-ready
microk8s.kubectl apply -k /project/k8s/overlays/single-node --namespace=build-service