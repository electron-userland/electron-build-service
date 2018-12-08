// Copyright (c) 2015 Joshua Marsh. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file in the root of the repository or at
// https://raw.githubusercontent.com/icub3d/gop/master/LICENSE.

// Package gopool implements a concurrent work processing model. It is
// a similar to thread pools in other languages, but it uses
// goroutines and channels. A pool is formed wherein several
// goroutines get tasks from a channel. Various sources can be used to
// schedule tasks and given some coordination gopools on various
// systems can work from the same source.
package gopool

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// JobEntry is a some type of work that the gopool should perform.
// The Stringer interface is used to aid in logging.
type JobEntry interface {
	fmt.Stringer
	// Run performs the work for this task. When the context is done,
	// processing should stop as soon as reasonably possible. Long
	// running tasks should make sure it's watching the context's Done()
	// channel.
	GetRunnable(context.CancelFunc) Runnable
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
	name       string
	jobChannel <-chan JobEntry
	wg         sync.WaitGroup
	context    context.Context

	// channel to ask worker to exit (but not to abort running jobs)
	closeChannel chan struct{}

	JobMaxTime      time.Duration
	RunningJobCount atomic.Uint32
}

// New creates a new GoPool with the given number of goroutines. The
// name is used for logging purposes. The goroutines are started as
// part of calling New().
//
// The goroutines will stop when the given context is done. If you
// want to make sure all of the tasks have got the signal and stopped
// cleanly, you should use Wait().
//
// The src channel is where the goroutines look for tasks.
func New(workerCount int, ctx context.Context, jobChannel <-chan JobEntry, logger *zap.Logger) *GoPool {
	p := &GoPool{
		jobChannel: jobChannel,
		context:    ctx,
	}

	for x := 0; x < workerCount; x++ {
		go p.worker(logger.With(zap.Int("worker", x)))
	}
	p.wg.Add(workerCount)
	return p
}

// Done returns channel until all of the workers have stopped. This won'job ever
// return if the context for this gopool is never done.
func (t *GoPool) Done() chan struct{} {
	c := make(chan struct{})
	go func() {
		defer close(c)
		t.Wait()
	}()
	return c
}

func (t *GoPool) Wait() {
	t.wg.Wait()
}

// String implements the fmt.Stringer interface. It just prints the
// name given to New().
func (t *GoPool) String() string {
	return t.name
}

func (t *GoPool) Close() {
	close(t.closeChannel)
}

// Worker is the function each goroutine uses to get and perform tasks.
// It stops when the stop channel is closed. It also stops if the source channel is closed but logs a message in addition.
func (t *GoPool) worker(logger *zap.Logger) {
	defer t.wg.Done()
	var jobCancel context.CancelFunc

	defer func() {
		if jobCancel != nil {
			jobCancel()
			jobCancel = nil
		}
	}()

	for {
		select {
		case <-t.closeChannel:
			logger.Debug("pool closed: stopping")
			return
		case <-t.context.Done():
			logger.Debug("stop channel closed: stopping")
			return
		case job, ok := <-t.jobChannel:
			if !ok {
				logger.Debug("input source closed: stopping")
				return
			}

			if t.context.Err() != nil {
				return
			}

			var jobContext context.Context
			logger.Debug("starting job", zap.Stringer("job", job))
			jobContext, jobCancel = context.WithTimeout(t.context, t.JobMaxTime)

			runnable := job.GetRunnable(jobCancel)
			if runnable == nil {
				logger.Debug("job is cancelled", zap.Stringer("job", job))
				continue
			}

			start := time.Now()
			t.RunningJobCount.Inc()
			runnable.Run(jobContext)
			t.RunningJobCount.Dec()
			jobCancel()
			jobCancel = nil
			logger.Debug("finished job", zap.Stringer("job", job), zap.Duration("duration", time.Since(start)))
		}
	}
}
