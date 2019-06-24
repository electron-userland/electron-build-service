package main

import (
	"fmt"
	"github.com/develar/app-builder/pkg/util"
	"net/http"
	"sort"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/electronuserland/electron-build-service/internal/agentRegistry"
	"go.uber.org/zap"
)

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
	_, _ = fmt.Fprintf(w, `{"endpoint": "https://%s"}`, agent.Address)
}

func configureRouter(logger *zap.Logger, disposer *Disposer) {
	limit := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	limit.SetBurst(10)

	a := agentRegistry.NewAgentRegistry(logger)
	disposer.Add(func() {
		util.Close(a)
	})
	http.Handle("/find-build-agent", tollbooth.LimitHandler(limit, &AgentRouter{
		agentRegistry: a,
		logger:        logger,
	}))
}

func getWeight(agent agentRegistry.BuildAgent) int {
	return agent.JobCount / agent.CpuCount
}

type byWeight []agentRegistry.BuildAgent

func (v byWeight) Len() int           { return len(v) }
func (v byWeight) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v byWeight) Less(i, j int) bool { return getWeight(v[i]) < getWeight(v[j]) }
