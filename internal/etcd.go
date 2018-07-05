package internal

import (
  "os"
  "time"

  "github.com/coreos/etcd/clientv3"
  "github.com/develar/errors"
  )

func CreateEtcdClient() (*clientv3.Client, error) {
  // https://github.com/kubernetes/kubernetes/blob/06e3fefc2153637daa65657025794b7dc27f6f33/staging/src/k8s.io/apiserver/pkg/storage/storagebackend/factory/etcd3.go#L32-L57
  // https://github.com/coreos/etcd/issues/9495
  client, err := clientv3.New(clientv3.Config{
    Endpoints: []string{getEtcdEndpoint()},
    // allow to wait when etcd container will be started
    DialKeepAliveTime:    30 * time.Second,
    DialKeepAliveTimeout: 10 * time.Second,
  })
  return client, errors.WithStack(err)
}

func getEtcdEndpoint() string {
  endpoint := os.Getenv("ETCD_ENDPOINT")
  // etcd-operator creates this service discovery entry by default (https://github.com/coreos/etcd-operator/blob/master/doc/user/client_service.md),
  // so, defaults provided for k8s (rancher), for docker env can be used to customize
  if endpoint == "" {
    return "http://etcd-cluster-client:2379"
  } else {
    return endpoint
  }
}