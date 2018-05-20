package internal

import (
  "runtime"

  "github.com/develar/errors"
  "go.uber.org/zap"
)

func MapAsync(taskCount int, logger *zap.Logger, taskProducer func(taskIndex int) (func() error, error)) error {
  return MapAsyncConcurrency(taskCount, runtime.NumCPU()+1, logger, taskProducer)
}

func MapAsyncConcurrency(taskCount int, concurrency int, logger *zap.Logger, taskProducer func(taskIndex int) (func() error, error)) error {
  if taskCount == 0 {
    return nil
  }

  logger.Debug("map async", zap.Int("taskCount", taskCount))

  if taskCount == 1 {
    task, err := taskProducer(0)
    if err != nil {
      return errors.WithStack(err)
    }
    if task != nil {
      return errors.WithStack(task())
    }
    return nil
  }

  errorChannel := make(chan error)
  doneChannel := make(chan bool, taskCount)
  quitChannel := make(chan bool)

  sem := make(chan bool, concurrency)
  for i := 0; i < taskCount; i++ {
    // wait semaphore
    sem <- true

    task, err := taskProducer(i)
    if err != nil {
      close(quitChannel)
      return errors.WithStack(err)
    }

    if task == nil {
      <-sem
      doneChannel <- true
      continue
    }

    go func(task func() error) {
      defer func() {
        // release semaphore, notify done
        <-sem
        doneChannel <- true
      }()

      // select waits on multiple channels, if quitChannel is closed, read will succeed without blocking
      // the default case in a select is run if no other case is ready
      select {
      case <-quitChannel:
        return

      default:
        err := task()
        if err != nil {
          errorChannel <- errors.WithStack(err)
        }
      }
    }(task)
  }

  finishedCount := 0
  for {
    select {
    case err := <-errorChannel:
      close(quitChannel)
      return err

    case <-doneChannel:
      finishedCount++
      if finishedCount == taskCount {
        return nil
      }
    }
  }
}
