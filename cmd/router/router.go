package main

import (
  "fmt"
  "net/http"
  "sort"
  "time"

  "github.com/develar/errors"
  "github.com/didip/tollbooth"
  "github.com/didip/tollbooth/limiter"
  "github.com/electronuserland/electron-build-service/internal"
  "github.com/electronuserland/electron-build-service/internal/agentRegistry"
  "go.uber.org/zap"
)

func main() {
  logger := internal.CreateLogger()
  defer logger.Sync()

  err := start(logger)
  if err != nil {
    logger.Fatal("cannot start", zap.Error(err))
  }
}

type AgentRouter struct {
  agentRegistry *agentRegistry.AgentRegistry
  logger        *zap.Logger
}

func (t *AgentRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  agentMap, err := t.agentRegistry.GetAgents()
  if err != nil {
    t.logger.Error("cannot get agents", zap.Error(err))
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
    return
  }

  if len(agentMap) == 0 {
    errorMessage := "no running build agents"
    t.logger.Error(errorMessage)
    http.Error(w, errorMessage, http.StatusServiceUnavailable)
    return
  }

  agents := make([]agentRegistry.BuildAgent, len(agentMap))
  i := 0
  for _, v := range agentMap {
    agents[i] = *v
    i++
  }

  if len(agents) > 1 {
    sort.Sort(byWeight(agents))
  }

  agent := agents[0]

  if agent.JobCount > 16 {
    errorMessage := "all build agents are overloaded"
    t.logger.Error(errorMessage)
    http.Error(w, errorMessage, http.StatusServiceUnavailable)
    return
  }

  w.Header().Set("Content-Type", "application/json; charset=utf-8")
  w.WriteHeader(http.StatusOK)
  fmt.Fprintf(w, `{"endpoint": "https://%s"}`, agent.Address)
}

func start(logger *zap.Logger) error {
  registry, err := agentRegistry.NewAgentRegistry(logger)
  if err != nil {
    return errors.WithStack(err)
  }

  err = registry.Listen()
  if err != nil {
    return errors.WithStack(err)
  }
  defer internal.Close(registry, logger)

  limit := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
  limit.SetBurst(10)

  http.Handle("/find-build-agent", tollbooth.LimitHandler(limit, &AgentRouter{
    agentRegistry: registry,
    logger:        logger,
  }))

  port := internal.GetListenPort("ROUTER_PORT")
  logger.Info("started", zap.String("port", port))
  internal.ListenAndServeTLS(port, 5 * time.Second, logger)
  logger.Info("stopped")
  return nil
}

func getWeight(agent agentRegistry.BuildAgent) int {
  return agent.JobCount / agent.CpuCount
}

type byWeight []agentRegistry.BuildAgent

func (v byWeight) Len() int           { return len(v) }
func (v byWeight) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v byWeight) Less(i, j int) bool { return getWeight(v[i]) < getWeight(v[j]) }