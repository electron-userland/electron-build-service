package internal

import (
  "crypto/x509"
  "io/ioutil"
  "time"

  "github.com/coreos/etcd/clientv3"
  "github.com/develar/errors"
  "go.uber.org/zap"
)

func CreateEtcdClient(logger *zap.Logger) (*clientv3.Client, error) {
  caBytes, err := ioutil.ReadFile("/run/secrets/bundle.crt")
  if err != nil {
    return nil, errors.WithStack(err)
  }

  caCertPool := x509.NewCertPool()
  caCertPool.AppendCertsFromPEM(caBytes)

  client, err := clientv3.New(clientv3.Config{
    Endpoints: []string{"http://etcd-1:2379", "http://etcd-2:2379", "http://etcd-3:2379"},
    // allow to wait when etcd container will be started
    DialTimeout:          10 * time.Second,
    AutoSyncInterval:     1 * time.Minute,
    DialKeepAliveTime:    1 * time.Minute,
    DialKeepAliveTimeout: 5 * time.Second,
    //TLS: &tls.Config{
    //  ServerName: "electron.build.local",
    //  RootCAs:    caCertPool,
    //},
  })
  return client, errors.WithStack(err)
}
