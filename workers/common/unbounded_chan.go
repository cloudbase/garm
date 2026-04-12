// Copyright 2026 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.

package common

import "context"

// UnboundedChan provides an unbounded FIFO channel backed by an internal
// slice-based queue. A router goroutine absorbs sends into the queue
// immediately and feeds them to the output one at a time. This ensures
// the producer is never blocked (beyond a brief channel send), while
// the consumer processes events sequentially.
// In theory, this does have the potential to grow forever but in practice
// it should not happen as we consume the queue and process it. Regardless
// of result, the queue should shrink. The only way this never shrinks is if
// the consumer is deadlocked for whatever reason.
//
// Use In() to send events and Out() to receive them.
type UnboundedChan[T any] struct {
	in   chan T
	out  chan T
	ctx  context.Context
	quit <-chan struct{}
}

// NewUnboundedChan creates and starts an UnboundedChan. The router goroutine
// runs until ctx is cancelled or quit is closed.
func NewUnboundedChan[T any](ctx context.Context, quit <-chan struct{}) *UnboundedChan[T] {
	u := &UnboundedChan[T]{
		in:   make(chan T),
		out:  make(chan T),
		ctx:  ctx,
		quit: quit,
	}
	go u.run()
	return u
}

// In returns the send side of the channel.
func (u *UnboundedChan[T]) In() chan<- T {
	return u.in
}

// Out returns the receive side of the channel.
func (u *UnboundedChan[T]) Out() <-chan T {
	return u.out
}

// Process reads from the output channel and calls handler for each item
// sequentially. It blocks until ctx is cancelled or quit is closed.
func (u *UnboundedChan[T]) Process(handler func(T)) {
	for {
		select {
		case item := <-u.out:
			handler(item)
		case <-u.ctx.Done():
			return
		case <-u.quit:
			return
		}
	}
}

func (u *UnboundedChan[T]) run() {
	var queue []T

	for {
		if len(queue) == 0 {
			select {
			case item := <-u.in:
				queue = append(queue, item)
			case <-u.ctx.Done():
				return
			case <-u.quit:
				return
			}
			continue
		}

		select {
		case item := <-u.in:
			queue = append(queue, item)
		case u.out <- queue[0]:
			queue = queue[1:]
		case <-u.ctx.Done():
			return
		case <-u.quit:
			return
		}
	}
}
