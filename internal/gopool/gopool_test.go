// Copyright (c) 2015 Joshua Marsh. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file in the root of the repository or at
// https://raw.githubusercontent.com/icub3d/gop/master/LICENSE.

package gopool

import (
			"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/electronuserland/electron-build-service/internal"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func TestGoPool(t *testing.T) {
	var l sync.Mutex
	var items []int
	var exp []int
	ai := func(i int) {
		l.Lock()
		defer l.Unlock()
		items = append(items, i)
	}

	queue := NewPriorityQueue()
	ctx, cancel := context.WithCancel(context.Background())

	logger := internal.CreateLogger()

	managedSource := NewManagedSource(queue, ctx, logger)
	pool := New(5, ctx, managedSource.Source, logger)

	// Add a bunch of tasks and wait for them to finish.
	for x := 0; x < 500; x++ {
		managedSource.Add <- NewJob(&tt{f: ai, i: x}, 0)
		exp = append(exp, x)
	}
	for managedSource.PendingJobCount.Load() > 1 {
		time.Sleep(20 * time.Millisecond)
	}
	for x := 500; x < 1000; x++ {
		managedSource.Add <- NewJob(&tt{f: ai, i: x}, 0)
		exp = append(exp, x)
	}
	for queue.q.Len() > 1 {
		time.Sleep(20 * time.Millisecond)
	}

	// cleanup
	cancel()
	pool.Wait()

	// Verify the list.
	sort.Ints(items)
	if !reflect.DeepEqual(items, exp) {
		t.Errorf("Didn'job get all the items expected: %v, %v", len(items), len(exp))
	}
}

func TestGoPoolInputSourceClosed(t *testing.T) {
	src := make(chan JobEntry)
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Error(err)
		return
	}

	pool := New(5, context.Background(), src, logger)

	close(src)
	time.Sleep(10 * time.Millisecond)

	// Cleanup
	pool.Wait()
}

type tt struct {
	f func(int)
	i int
}

func (t *tt) String() string { return strconv.Itoa(t.i) }
func (t *tt) Run(ctx context.Context) {
	t.f(t.i)
}
