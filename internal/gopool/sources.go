package gopool

import (
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type ManagedSource struct {
	// channel where tasks can be retrieved
	source <-chan JobEntry
	// channel on which tasks can be added
	add chan<- JobEntry

	pendingJobCount atomic.Uint32
}

// creates a managed source using the given Sourcer and starts it
func newManagedSource(queue *PriorityQueue, ctx context.Context, logger *zap.Logger) *ManagedSource {
	outputChannel := make(chan JobEntry)
	addChannel := make(chan JobEntry)

	source := &ManagedSource{
		source: outputChannel,
		add:    addChannel,
	}

	go func() {
		updatePendingJobCount := func() {
			source.pendingJobCount.Store(uint32(queue.Length()))
		}

		var topJob JobEntry

		defer func() {
			close(outputChannel)

			if topJob != nil {
				topJob.Cancel()
				topJob = nil
			}

			for {
				job := queue.Next()
				if job == nil {
					break
				}

				job.Cancel()
			}

			updatePendingJobCount()
		}()

		for {
			if topJob == nil {
				topJob = queue.Next()
				updatePendingJobCount()
			}

			// setup outputChannel based on the availability of a task
			var currentOutputChannel chan JobEntry
			if topJob == nil {
				currentOutputChannel = nil
			} else {
				currentOutputChannel = outputChannel
			}

			select {
			case task, ok := <-addChannel:
				if !ok {
					addChannel = nil
					logger.Debug("addChannel closed, no longer selecting with it")
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
