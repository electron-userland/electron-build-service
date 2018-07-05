package agentRegistry

import (
  "context"
  "runtime"
  "time"

  "github.com/coreos/etcd/clientv3"
  "github.com/develar/errors"
  "github.com/electronuserland/electron-build-service/internal"
  "go.uber.org/zap"
)

const entryTtl = 1 * time.Minute

type AgentEntry struct {
  Key    string
  store  *clientv3.Client

  timer *time.Timer
  isClosed chan bool
  leaseId clientv3.LeaseID

  logger *zap.Logger
}

// ttlInSeconds - is the server selected time-to-live, in seconds, for the lease
func computeRenewLeaseTimerDuration(ttlInSeconds int64) time.Duration {
  return time.Duration(ttlInSeconds - 5) * time.Second
}

func NewAgentEntry(key string, logger *zap.Logger) (*AgentEntry, error) {
  logger.Info("register agent")
  store, err := internal.CreateEtcdClient()
  if err != nil {
    return nil, errors.WithStack(err)
  }

  leaseGrantResponse, err := store.Grant(context.Background(), int64(entryTtl/time.Second))
  if err != nil {
    return nil, errors.WithStack(err)
  }

  timer := time.NewTimer(computeRenewLeaseTimerDuration(leaseGrantResponse.TTL))
  isClosed := make(chan bool, 1)

  go func() {
    for {
      select {
      case <-isClosed:
        return
      case <-timer.C:
        response, err := store.KeepAliveOnce(context.Background(), leaseGrantResponse.ID)
        if err != nil {
          logger.Error("cannot renew the agent entry lease", zap.String("key", key), zap.Error(err))
        }

        timer.Reset(computeRenewLeaseTimerDuration(response.TTL))
      }
    }
  }()

  // job count (cannot be more than 127 (actually, router limits to 16 and then returns 503 (and client retry request after at least 30 seconds)))
  _, err = store.Put(context.Background(), key, string([]byte{byte(runtime.NumCPU()), 0}), clientv3.WithLease(leaseGrantResponse.ID))
  if err != nil {
    return nil, errors.WithStack(err)
  }

  entry := &AgentEntry{
    Key:    key,
    logger: logger,
    store:  store,

    leaseId:  leaseGrantResponse.ID,
    timer:    timer,
    isClosed: isClosed,
  }
  return entry, nil
}

func (t *AgentEntry) Update(jobCount int) {
  _, err := t.store.Put(context.Background(), t.Key, string([]byte{byte(runtime.NumCPU()), byte(jobCount)}), clientv3.WithLease(t.leaseId))
  if err != nil {
    t.logger.Error("cannot update job", zap.String("key", t.Key), zap.Error(err))
  }
}

func (t *AgentEntry) Close() error {
  t.logger.Info("unregister agent")
  defer internal.Close(t.store, t.logger)

  t.isClosed <- true
  t.timer.Stop()

  _, err := t.store.Revoke(context.Background(), t.leaseId)
  if err != nil {
    return errors.WithStack(err)
  }
  return nil
}
