package main

import "sync"

type Disposer struct {
	disposables []func()

	lock *sync.Mutex
}

func NewDisposer() *Disposer {
	return &Disposer{lock: &sync.Mutex{}}
}

func (t *Disposer) Dispose() {
	t.lock.Lock()
	defer t.lock.Unlock()

	disposables := t.disposables

	if disposables == nil {
		return
	}

	t.disposables = nil
	for _, closeListener := range disposables {
		closeListener()
	}
}

func (t *Disposer) Add(disposable func()) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.disposables = append(t.disposables, disposable)
}
