package internal

import (
	"os"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/develar/errors"
)

// The short keepalive timeout and interval have been chosen to aggressively
// detect a failed etcd server without introducing much overhead.
const keepaliveTime = 30 * time.Second
const keepaliveTimeout = 10 * time.Second

// dialTimeout is the timeout for failing to establish a connection.
const dialTimeout = 20 * time.Second

func CreateEtcdClient() (*clientv3.Client, error) {
	etcdEndpoint := getEtcdEndpoint()

	// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apiserver/pkg/storage/storagebackend/factory/etcd3.go
	// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apiserver/pkg/storage/storagebackend/factory/etcd3.go#L100
	// https://github.com/coreos/etcd/issues/9495
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: dialTimeout,
		// allow to wait when etcd container will be started
		DialKeepAliveTime:    keepaliveTime,
		DialKeepAliveTimeout: keepaliveTimeout,
	})
	return client, errors.WithStack(err)
}

func getEtcdEndpoint() string {
	endpoint := os.Getenv("ETCD_ENDPOINT")
	// etcd-operator creates this service discovery entry by default (https://github.com/coreos/etcd-operator/blob/master/doc/user/client_service.md),
	// so, defaults provided for k8s (rancher), for docker env can be used to customize
	if endpoint == "" {
		return "http://etcd-client:2379"
	} else {
		return endpoint
	}
}
