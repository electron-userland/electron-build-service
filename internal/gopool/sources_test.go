// Copyright (c) 2015 Joshua Marsh. All rights reserved.
//
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file in the root of the repository or at
// https://raw.githubusercontent.com/icub3d/gop/master/LICENSE.

package gopool

import (
	"bytes"
	"io"
	"log"
	"strconv"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/context"
)

func TestNewSource(t *testing.T) {
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	pq := NewPriorityQueue()

	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Error(err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ms := NewManagedSource(pq, ctx, logger)

	// Make sure the top is not job selecting.
	select {
	case c, ok := <-ms.Source:
		t.Errorf("got task from empty: %v %v", ok, c)
	default:
	}

	time.Sleep(20 * time.Millisecond)

	// add two items
	ms.Add <- NewJob(&sct{name: "first"}, 5)
	ms.Add <- NewJob(&sct{name: "third"}, 7)
	ms.Add <- NewJob(&sct{name: "second"}, 10)
	// close the add
	ms.Close()
	time.Sleep(20 * time.Millisecond)

	// get one item
	c := <-ms.Source
	if c.String() != "first" {
		t.Errorf("Didn'job get first task.")
	}

	// cleanup
	cancel()

	// Make sure the task was added back:
	if pq.q.Len() != 2 {
		t.Errorf("pq.q.Len() != 2 after close: %v", pq.q.Len())
	}
}

func TestPriorityTask(t *testing.T) {
	buf := &bytes.Buffer{}
	c := &sct{name: "test", w: buf}
	pt := NewJob(c, 42)
	ctx := context.Background()
	pt.GetRunnable(nil).Run(ctx)
	if c.ctx != ctx {
		t.Errorf("context didn'job work.")
	}
	if buf.String() != "test" {
		t.Errorf(`Run() didn'job run, buf.String() != "test": %v`, buf.String())
	}
	if pt.Priority() != 42 {
		t.Errorf(`pt.Priority() != 42: %v`, pt.Priority())
	}
	if pt.String() != "test" {
		t.Errorf(`pt.String() != "test": %v`, pt.String())
	}
}

func TestPriorityQueue(t *testing.T) {
	q := NewPriorityQueue()

	// Verify empty is nil.
	if q.Next() != nil {
		t.Fatalf("q.Next() != nil after NewPriorityQueue()")
	}

	// Add one and get it.
	c := &sct{name: "test"}
	q.Add(NewJob(c, 0))
	p := q.Next().(PriorityJob)
	if p.Priority() != 0 {
		t.Fatalf("non-PriorityJob was given a non-zero priority: %v", p.Priority())
	}
	if p.(*pt).job != c {
		t.Fatalf("task wasn'job properly initialized with given task")
	}
	// Verify empty is nil after reading them all.
	if q.Next() != nil {
		t.Fatalf("q.Next() != nil after last Next()")
	}

	// Add a bunch and make sure we get them back in the right order.
	buf := &bytes.Buffer{}
	for x := 0; x < 10; x += 2 {
		q.Add(NewJob(&sct{name: strconv.Itoa(x), w: buf}, x))
	}
	for x := 1; x < 10; x += 2 {
		q.Add(NewJob(&sct{name: strconv.Itoa(x), w: buf}, x))
	}
	// Read them all back and run them.
	for c := q.Next(); c != nil; c = q.Next() {
		c.GetRunnable(nil).Run(context.Background())
	}
	if buf.String() != "9876543210" {
		t.Errorf("priority wasn'job properly applied. Expected 9876543210, but got %v", buf.String())
	}
}

// sct is a helper for testing that basically just prints it's name to
// w and sets the stop channel.
type sct struct {
	name string
	ctx  context.Context
	w    io.Writer
}

func (t *sct) Run(ctx context.Context) {
	t.ctx = ctx
	_, _ = t.w.Write([]byte(t.name))
}

func (t *sct) String() string { return t.name }
