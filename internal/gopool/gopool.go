// Copyright (c) 2015 Joshua Marsh. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file in the root of the repository or at
// https://raw.githubusercontent.com/icub3d/gop/master/LICENSE.
package gopool

import (
	"fmt"
	"go.uber.org/atomic"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// JobEntry is a some type of work that the gopool should perform.
// The Stringer interface is used to aid in logging.
type JobEntry interface {
	fmt.Stringer

	Priority() int

	// can be called several times
	Cancel()

	Run(jobContext context.Context, cancelFunc context.CancelFunc)
}

type Runnable interface {
	fmt.Stringer

	// Run performs the work for this task.
	// When the context is done, processing should stop as soon as reasonably possible.
	// Long running tasks should make sure it's watching the context's Done() channel.
	Run(context.Context)
}

// GoPool is a group of goroutines that work on Tasks. Each goroutine
// gets work from a channel until the context signals that it's done.
type GoPool struct {
	name string

	waitGroup sync.WaitGroup
	context   context.Context

	queue *ManagedSource

	pendingJobCount atomic.Int32
	runningJobCount atomic.Int32

	JobMaxTime time.Duration

	closeOnce sync.Once

	// channel to ask worker to exit (but not to abort running jobs)
	closeChannel chan struct{}
}

func (t *GoPool) AddJob(job Runnable, priority int) JobEntry {
	jobEntry := newJob(job, priority)
	t.queue.add <- jobEntry
	return jobEntry
}

func (t *GoPool) GetPendingJobCount() int {
	return int(t.queue.pendingJobCount.Load())
}

func (t *GoPool) GetRunningJobCount() int {
	return int(t.runningJobCount.Load())
}

// New creates a new GoPool with the given number of goroutines.
// The goroutines are started as part of calling New().
//
// The goroutines will stop when the given context is done. If you
// want to make sure all of the tasks have got the signal and stopped
// cleanly, you should use Wait().
func New(workerCount int, ctx context.Context, logger *zap.Logger) *GoPool {
	managedSource := newManagedSource(NewPriorityQueue(), ctx, logger)
	pool := &GoPool{
		context: ctx,
		queue:   managedSource,

		closeChannel: make(chan struct{}),
	}

	for index := 0; index < workerCount; index++ {
		go pool.worker(logger.With(zap.Int("worker", index)))
	}
	return pool
}

// Done returns channel until all of the workers have stopped. This won'job ever return if the context for this gopool is never done.
func (t *GoPool) Done() chan struct{} {
	c := make(chan struct{})
	go func() {
		defer close(c)
		t.Wait()
	}()
	return c
}

func (t *GoPool) Wait() {
	t.waitGroup.Wait()
}

// String implements the fmt.Stringer interface. It just prints the name given to New().
func (t *GoPool) String() string {
	return t.name
}

func (t *GoPool) Close() {
	t.closeOnce.Do(func() {
		// close queue to not accept new jobs
		close(t.queue.add)
		close(t.closeChannel)
	})
}

// Worker is the function each goroutine uses to get and perform tasks.
// It stops when the stop channel is closed. It also stops if the source channel is closed but logs a message in addition.
func (t *GoPool) worker(logger *zap.Logger) {
	t.waitGroup.Add(1)
	stopReason := "unknown"
	defer func() {
		t.waitGroup.Done()
		logger.Debug("stopping", zap.String("reason", stopReason))
	}()

	for {
		select {
		case <-t.closeChannel:
			stopReason = "pool closed"
			return
		case <-t.context.Done():
			stopReason = "stop channel closed"
			return
		case job, ok := <-t.queue.source:
			if !ok {
				stopReason = "input source closed"
				return
			}

			contextError := t.context.Err()
			if contextError != nil {
				logger.Debug("stopping", zap.NamedError("reason", contextError))
				return
			}

			t.executeJob(logger.With(zap.Stringer("job", job)), job)
		}
	}
}

func (t *GoPool) executeJob(logger *zap.Logger, job JobEntry) {
	logger.Debug("starting job")
	jobContext, jobCancel := context.WithTimeout(t.context, t.JobMaxTime)

	start := time.Now()

	var closeOnce sync.Once

	cancelFuncWrapper := func() {
		closeOnce.Do(func() {
			t.runningJobCount.Dec()
			isCancelled := jobContext.Err() != nil

			defer func() {
				if logger.Core().Enabled(zap.DebugLevel) {
					var suffix string
					if isCancelled {
						suffix = "cancelled"
					} else {
						suffix = "finished"
					}
					logger.Debug("job "+suffix, zap.Duration("duration", time.Since(start)))
				}
			}()

			jobCancel()
		})
	}

	t.runningJobCount.Inc()
	defer cancelFuncWrapper()
	job.Run(jobContext, cancelFuncWrapper)
}
