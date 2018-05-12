package agentRegistry

import (
  "context"
  "strings"
  "sync"
  "time"

  "github.com/coreos/etcd/clientv3"
  "github.com/coreos/etcd/mvcc/mvccpb"
  "github.com/develar/errors"
  "github.com/electronuserland/electron-build-service/internal"
  "go.uber.org/zap"
)

// well, it is ok if client will get dead node address and will check health - but we will not DDoS our server and request new list on each client request
const agentListTtl = 5 * time.Minute

type AgentRegistry struct {
  lastUpdate  time.Time
  buildAgents map[string]*BuildAgent

  mutex sync.RWMutex

  store  *clientv3.Client
  logger *zap.Logger
}

func NewAgentRegistry(logger *zap.Logger) (*AgentRegistry, error) {
  store, err := internal.CreateEtcdClient()
  if err != nil {
    return nil, errors.WithStack(err)
  }

  result := &AgentRegistry{
    store:  store,
    logger: logger,
  }
  return result, nil
}

func (t *AgentRegistry) Listen() error {
  watchChain := t.store.Watch(context.Background(), "/builders/", clientv3.WithPrefix())
  go func() {
    for response := range watchChain {
      t.handleEvents(&response)
    }
  }()
  return nil
}

// invalidate cache on any PUT/DELETE events
// lastUpdate time is not updated - better to perform full update to avoid dead entries
func (t *AgentRegistry) handleEvents(response *clientv3.WatchResponse)  {
  t.mutex.Lock()
  defer t.mutex.Unlock()

  if t.buildAgents == nil {
    return
  }

  for _, event := range response.Events {
    key := etcdKeyToOurMapKey(event.Kv)
    if event.Type == mvccpb.PUT {
      info, found := t.buildAgents[key]
      if found {
        oldJobCount := info.JobCount
        // already known agent
        applyToInfo(info, event.Kv)
        t.logger.Info("agent updated",
          zap.String("key", key),
          zap.Int("oldJobCount", oldJobCount),
          zap.Int("newJobCount", info.JobCount),
        )
      } else {
        t.logger.Info("agent added", zap.String("key", key))
        // new agent - for the sake of simplicity, for now simply invalidate our state
        t.buildAgents = nil
        break
      }
    } else {
      t.logger.Info("agent removed", zap.String("key", key))
      // DELETE
      delete(t.buildAgents, key)
    }
  }
}

func (t *AgentRegistry) getListIfValid(isLocked bool) map[string]*BuildAgent {
  if !isLocked {
    t.mutex.RLock()
    defer t.mutex.RUnlock()
  }

  if t.buildAgents == nil {
    t.logger.Debug("no valid agent list", zap.String("reason", "buildAgents is nil"))
    return nil
  }

  since := time.Since(t.lastUpdate)
  if since < agentListTtl {
    return t.buildAgents
  } else {
    t.logger.Debug("no valid agent list", zap.String("reason", "buildAgents list outdated"), zap.Duration("since", since))
    return nil
  }
}

func (t *AgentRegistry) GetAgents() (map[string]*BuildAgent, error) {
  result := t.getListIfValid(false)
  if result != nil {
    return result, nil
  }

  t.mutex.Lock()
  defer t.mutex.Unlock()

  result = t.getListIfValid(true)
  if result != nil {
    return result, nil
  }

  response, err := t.store.Get(context.Background(), "/builders/", clientv3.WithPrefix())
  if err != nil {
    return nil, errors.WithStack(err)
  }

  result = make(map[string]*BuildAgent, len(response.Kvs))
  for _, keyValue := range response.Kvs {
    info := BuildAgent{}

    t.logger.Debug("etcd entry", zap.ByteString("key", keyValue.Key), zap.ByteString("value", keyValue.Value))

    info.Address = etcdKeyToOurMapKey(keyValue)
    applyToInfo(&info, keyValue)
    result[info.Address] = &info
  }

  t.buildAgents = result
  t.lastUpdate = time.Now()
  return result, nil
}

func applyToInfo(info *BuildAgent, keyValue *mvccpb.KeyValue) {
  info.CpuCount = int(keyValue.Value[0])
  info.JobCount = int(keyValue.Value[1])
}

func etcdKeyToOurMapKey(keyValue *mvccpb.KeyValue) string {
  key := string(keyValue.Key)
  return key[strings.LastIndex(key, "/") + 1:]
}

func (t *AgentRegistry) Close() error {
  return t.store.Close()
}

type BuildAgent struct {
  Address  string
  JobCount int
  CpuCount int
}
