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
	"golang.org/x/net/context"
)

func TestGoPool(t *testing.T) {
	var l sync.Mutex
	var items []int
	ai := func(i int) {
		l.Lock()
		defer l.Unlock()
		items = append(items, i)
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger := internal.CreateLogger()

	pool := New(5, ctx, logger)

	var exp []int
	// Add a bunch of tasks and wait for them to finish.
	for x := 0; x < 500; x++ {
		pool.AddJob(&tt{f: ai, i: x}, 0)
		exp = append(exp, x)
	}

	for pool.GetPendingJobCount() > 1 {
		time.Sleep(20 * time.Millisecond)
	}

	for x := 500; x < 1000; x++ {
		pool.AddJob(&tt{f: ai, i: x}, 0)
		exp = append(exp, x)
	}

	for pool.GetRunningJobCount() > 1 {
		time.Sleep(20 * time.Millisecond)
	}

	pool.Close()
	pool.Wait()

	// cleanup
	cancel()

	// Verify the list.
	sort.Ints(items)
	if !reflect.DeepEqual(items, exp) {
		t.Errorf("Didn'job get all the items expected: %v, %v", len(items), len(exp))
	}
}

type tt struct {
	f func(int)
	i int
}

func (t *tt) String() string { return strconv.Itoa(t.i) }
func (t *tt) Run(ctx context.Context) {
	t.f(t.i)
}
