// The MIT License (MIT)
//
// Copyright (c) 2018 Mervin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package pigeon

import (
	"errors"
	"net"
	"sync"

	"sync/atomic"
)

var NilListenerError = errors.New("listener is nil")
var NilCallbackError = errors.New("callback is nil")

type AcceptHandleFunc func(conn net.Conn)

// Acceptor is a interface wraps necessary methods for network connection acceptance.
// The implementation should be based on FSM.
type Acceptor interface {
	Lifecycle
	Sync
}

// AcceptorConfig is a data struct for acceptor initialization.
type AcceptorConfig struct {
	Parallelism      uint8
	Listener         net.Listener
	AcceptHandleFunc func(conn net.Conn)
}

// ParallelAcceptor is a implementation of Acceptor which provide connection parallel acceptance.
// After a new connection have been accepted, the accept callback function which user defined
// in AcceptorConfig will be invoked by acceptor goroutine.
type parallelAcceptor struct {
	parallelism      uint8
	listener         net.Listener
	acceptHandleFunc AcceptHandleFunc
	workerCount      uint32
	running          bool
	mu               sync.RWMutex
	wg               sync.WaitGroup
}

// Start only work on acceptor is not running. It will start goroutines for connection
// parallel acceptance.
func (a *parallelAcceptor) Start() error {

	if a.listener == nil {
		return NilListenerError
	}
	if a.acceptHandleFunc == nil {
		return NilCallbackError
	}

	// Mutex
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		// Only work on acceptor is not running.
		return nil
	}

	for i := uint8(0); i < a.parallelism; i++ {
		workerIndex := i
		go func() {
			Trace("AcceptWorker-%d for %s start.", workerIndex, a.listener.Addr().String())

			defer func() {
				a.decreaseWorkerCount()
				if a.loadWorkerCount() == 0 {
					a.running = false
					a.wg.Done()
				}
				Trace("AcceptWorker-%d for %s stop.", workerIndex, a.listener.Addr().String())
			}()

			for {
				conn, err := a.listener.(*net.TCPListener).AcceptTCP()
				if err != nil {
					return
				}
				a.acceptHandleFunc(conn)
			}
		}()
		a.increaseWorkerCount()
	}
	a.wg.Add(1)
	a.running = true
	return nil
}

// IsRunning returns true if acceptor is current running.
func (a *parallelAcceptor) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// Stop will close network listener which bind with acceptor
// and stop all parallel accept goroutine.
func (a *parallelAcceptor) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		a.listener.Close()
	}
}

// Sync block invoker goroutine until acceptor stop.
func (a *parallelAcceptor) Sync() {
	a.wg.Wait()
}

func (a *parallelAcceptor) increaseWorkerCount() {
	for {
		val := a.loadWorkerCount()
		if atomic.CompareAndSwapUint32(&a.workerCount, val, val+1) {
			return
		}
	}
}

func (a *parallelAcceptor) decreaseWorkerCount() {
	for {
		val := a.loadWorkerCount()
		if atomic.CompareAndSwapUint32(&a.workerCount, val, val-1) {
			return
		}
	}
}

func (a *parallelAcceptor) loadWorkerCount() uint32 {
	return atomic.LoadUint32(&a.workerCount)
}

// Create a new ParallelAcceptor with acceptor properties.
func NewAcceptor(config AcceptorConfig) Acceptor {
	return &parallelAcceptor{
		parallelism:      config.Parallelism,
		listener:         config.Listener,
		acceptHandleFunc: config.AcceptHandleFunc,
		running:          false,
	}
}
