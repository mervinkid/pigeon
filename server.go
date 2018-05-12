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
	"net"
	"sync"
)

// Server is the interface that wraps the basic method to implement a tcp network server based on FSM.
type Server interface {
	Lifecycle
	Sync
}

// PipelineServer is the default implementation of Server interface which using ParallelAcceptor for
// connection parallel acceptance and using DuplexPipeline for ease connection handling.
type pipelineServer struct {
	config ServerConfig

	// Initializer
	initializer PipelineInitializer

	// State control
	running    bool
	acceptor   Acceptor
	stateMutex sync.RWMutex
	waitGroup  sync.WaitGroup
	// Channel group
	channelGroup ChannelGroup
}

// Start will start server with specified address configuration.
func (s *pipelineServer) Start() error {

	// Mutex state
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if s.running {
		// Only work on standby.
		return nil
	}

	addr := new(net.TCPAddr)
	addr.IP = s.config.IP
	addr.Port = s.config.Port
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	s.waitGroup.Add(1)

	// Init channel group for channel management.
	channelGroup := NewHashSafeChannelGroup()
	s.channelGroup = channelGroup

	// Init and start acceptor
	ac := AcceptorConfig{}
	ac.Parallelism = s.config.AcceptorSize
	ac.Listener = listener
	ac.AcceptHandleFunc = s.handleAccept
	acceptor := NewAcceptor(ac)

	s.acceptor = acceptor
	acceptor.Start()

	s.running = true

	return nil
}

// Stop will stop current server and release network resource.
func (s *pipelineServer) Stop() {

	// Mutex state
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if !s.running {
		// Only work on running.
		return
	}

	// Close acceptor
	if LifecycleCheckRun(s.acceptor) {
		LifecycleStop(s.acceptor)
	}

	// Close channels
	s.channelGroup.CloseAll()

	// Update state
	s.acceptor = nil
	s.running = false
	s.waitGroup.Done()

}

// Sync will block current goroutine until server stop.
func (s *pipelineServer) Sync() {
	s.waitGroup.Wait()
}

// IsRunning test state of current server.
func (s *pipelineServer) IsRunning() bool {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.running
}

// startConnAcceptor accept new connection with new goroutine.
func (s *pipelineServer) handleAccept(conn net.Conn) {
	go func() {
		// Setup connection.
		s.config.ApplyTCP(conn.(*net.TCPConn))

		Trace("Accept connection from %s.\n", conn.RemoteAddr().String())

		// Init and start pipeline.
		if s.initializer == nil {
			Trace("Close connection between %s cause initializer is nil.\n", conn.RemoteAddr().String())
			s.closeConn(conn)
			return
		}
		pipeline, err := NewPipeline(conn, s.initializer)
		if err != nil {
			Trace("Pipeline init failure cause %s\n.", err.Error())
			s.closeConn(conn)
			return
		}
		if err := LifecycleStart(pipeline); err != nil {
			Trace("Pipeline for remote %s start failure cause %s.\n", conn.RemoteAddr().String(), err.Error())
			s.closeConn(conn)
			return
		}
		s.channelGroup.Add(pipeline.GetChannel())

		// Monitoring pipeline lifecycle.
		pipeline.Sync()
		s.channelGroup.Remove(pipeline.GetChannel())
	}()
}

// closeConn close specified TCP connection.
func (s *pipelineServer) closeConn(conn net.Conn) {
	if conn != nil {
		conn.Close()
		Trace("Close connection between %s.\n", conn.RemoteAddr().String())
	}
}

// NewServer init a new server instance with specified configuration and initializer.
func NewServer(cfg ServerConfig, initializer PipelineInitializer) Server {
	return &pipelineServer{
		config:      cfg,
		initializer: initializer,
		running:     false,
		acceptor:    nil,
	}
}
