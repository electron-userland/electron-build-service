// Copyright (c) 2015 Joshua Marsh. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file in the root of the repository or at
// https://raw.githubusercontent.com/icub3d/gop/master/LICENSE.

package gopool

import (
  "fmt"

  "go.uber.org/atomic"
  "go.uber.org/zap"
  "golang.org/x/net/context"
)

// Sourcer is the interface that allows a type to be run as a source
// that communicates appropriately with a gopool. If this Sourcer is
// used by a managed source, the Next() and Add() methods are
// synchronized internally, so as long as no other places are calling
// them, they won'job suffer from race conditions. If they might be
// called concurrently, it is the implementers responsibility to
// synchronize usage (e.g. through a mutex).
type Sourcer interface {
  fmt.Stringer

  // Next returns the next task from the source. It should return nil
  // if there is currently no work.
  Next() JobEntry

  // Add schedules a task. It also reschedules a task during cleanup
  // if a task was taken but was unable to be sent. As such, it should
  // be available until the ManagedSource using it returns from a call
  // to Wait().
  Add(t JobEntry)
}

// ManagedSource wraps a Sourcer in a goroutine that synchronizes
// access to the Sourcer.
type ManagedSource struct {
  // Source is the channel where tasks can be retrieved.
  Source <-chan JobEntry

  // Add is the channel on which tasks can be added.
  Add chan<- JobEntry

  PendingJobCount atomic.Uint32
}

// Wait blocks until the ManagedSource is done. If you want to ensure
// that the managed source has cleaned up completely, you should call
// this.
func (t *ManagedSource) Close() {
  close(t.Add)
}

// creates a managed source using the given Sourcer and starts it
func NewManagedSource(queue *PriorityQueue, ctx context.Context, logger *zap.Logger) *ManagedSource {
  outputChannel := make(chan JobEntry)
  inputChannel := make(chan JobEntry)

  source := &ManagedSource{
    Source: outputChannel,
    Add:    inputChannel,
  }

  go func() {
    updatePendingJobCount := func() {
      source.PendingJobCount.Store(uint32(queue.Length()))
    }

    defer func() {
      if inputChannel != nil {
        close(inputChannel)
      }
      updatePendingJobCount()
    }()

    var topJob JobEntry
    var currentOutputChannel chan JobEntry
    for {
      if topJob == nil {
        topJob = queue.Next()
        updatePendingJobCount()
      }

      // setup outputChannel based on the availability of a task
      currentOutputChannel = outputChannel
      if topJob == nil {
        logger.Debug("no task available, none will be sent")
        currentOutputChannel = nil
      }

      select {
      case task := <-inputChannel:
        if task == nil {
          inputChannel = nil
          logger.Debug("inputChannel closed, no longer selecting with it")
          // do not return, process to send jobs to outputChannel
          continue
        }

        logger.Debug("add job", zap.Stringer("job", task))
        queue.Add(task)
        updatePendingJobCount()
        // we cannot send job to outputChannel here because send is blocking, but we cannot block since we need to read new jobs from inputChannel,
        // because if we will not read from inputChannel, send to inputChannel will be blocked but it is not what client expects (add job should be not blocking)

      case <-ctx.Done():
        logger.Info("stop requested")
        if topJob != nil {
          logger.Info("add back job", zap.Stringer("job", topJob))
          queue.Add(topJob)
          updatePendingJobCount()
          topJob = nil
        }
        return

      case currentOutputChannel <- topJob:
        logger.Debug("sent job", zap.Stringer("job", topJob))
        topJob = nil
      }
    }
  }()

  return source
}
