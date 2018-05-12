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
)

// Chan buffer
const (
	dataChanSize = 10
	cmdChanSize  = 2
)

// State of pipeline
const (
	stateNew      = iota
	stateReady
	stateRunning
	stateShutdown
)

// Buffer size
const (
	readBufferSize = 1024
	byteBufferSize = 2 * readBufferSize
)

// Errors
var (
	NilInitializerError = errors.New("initializer is nil")
	NilConnError        = errors.New("conn is nil")
	NilDecoderError     = errors.New("decoder is nil")
	NilEncoderError     = errors.New("encoder is nil")
	NilHandlerError     = errors.New("handler is nil")
)

type outboundEntity struct {
	Data     interface{}
	Callback func(err error)
}

// Pipeline is the interface defined necessary methods which makes a pipeline of FrameDecoder,
// FrameEncoder, and ChannelHandler for inbound and outbound data processing.
//
// Model:
//  +---------------------------------------+
//  |         TCP Network Connection        |
//  +---------------------------------------+
//          ↑(write)               ↓(read)
//  +----------------+     +----------------+
//  |  FrameEncoder  |     |  FrameDecoder  |
//  +----------------+     +----------------+
//          ↑(outbound)            ↓(inbound)
//  +----------------+     +----------------+
//  |    Channel     |     | ChannelHandler |
//  +----------------+     +----------------+
//
// State:
//  +-----+           +---------+          +----------+
//  | NEW | → Start → | RUNNING | → Stop → | SHUTDOWN |
//  +-----+           +---------+          +----------+
//
// Notes:
// The implementations should be parallel safe and based on FSM.
type Pipeline interface {
	Lifecycle
	Sync
	Sender
	GetChannel() Channel
	Remote() net.Addr
}

// DuplexPipeline is a implementation of Pipeline based on FSM and provide full duplex and
// non blocking processing for inbound and outbound data. Each pipeline will create three
// goroutine for data processing after start.
//
// Model:
//  +----------------------------------------------+
//  |            TCP Network Connection            |
//  +----------------------------------------------+
//          ↑(write)                      ↓(read)
//  +----------------+            +----------------+
//  |  FrameEncoder  |            |  FrameDecoder  |
//  +----------------+            +----------------+
//          ↑(push)                       ↓(produce)
//  +----------------+            +----------------+
//  | OutboundWorker |            | InboundBuffer  |
//  +----------------+            +----------------+
//          ↑(consume)                    ↓(consume)
//  +----------------+            +----------------+
//  | OutboundBuffer |            | InboundWorker  |
//  +----------------+            +----------------+
//          ↑(produce)                     ↓(push)
//  +----------------+            +----------------+
//  |    Channel     | ← relate → | ChannelHandler |
//  +----------------+            +----------------+
//
// State:
//  +-----+          +-------+           +---------+          +----------+
//  | NEW | → Init → | READY | → Start → | RUNNING | → Stop → | SHUTDOWN |
//  +-----+          +-------+           +---------+          +----------+
//
// Notes:
// Stop the pipeline will also close the tcp connection which bind with pipeline.
type duplexPipeline struct {
	encoder FrameEncoder
	decoder FrameDecoder
	handler ChannelHandler

	// Props
	conn    net.Conn // Setup while construct.
	channel Channel  // Setup after init.

	// State
	state          uint8
	stateMutex     sync.RWMutex
	stateWaitGroup sync.WaitGroup

	// Data chan
	inboundDataC  chan interface{}
	outboundDataC chan outboundEntity

	// Handler command chan
	inboundHandlerStopC  chan uint8
	outboundHandlerStopC chan uint8

	// Handler coroutine
	connReadSyncC chan uint8
	inboundSyncC  chan uint8
	outboundSyncC chan uint8
}

// NewPipeline create and init pipeline with initializer.
func NewPipeline(conn net.Conn, initializer PipelineInitializer) (Pipeline, error) {

	// Check arguments
	if conn == nil {
		return nil, NilConnError
	}
	if initializer == nil {
		return nil, NilInitializerError
	}

	// Init encoder, decoder and handler
	decoder := initializer.InitDecoder()
	Trace("Init decoder for %s.\n", conn.RemoteAddr())
	encoder := initializer.InitEncoder()
	Trace("Init encoder for %s.\n", conn.RemoteAddr())
	handler := initializer.InitHandler()
	Trace("Init handler for %s.\n", conn.RemoteAddr())

	// New pipeline
	pipeline := &duplexPipeline{
		conn:    conn,
		decoder: decoder,
		encoder: encoder,
		handler: handler,
	}

	// Init pipeline
	if err := pipeline.Init(); err != nil {
		return nil, err
	}

	return pipeline, nil
}

// GetChannel returns the channel which created and bind with pipeline.
func (p *duplexPipeline) GetChannel() Channel {
	return p.channel
}

// Remote returns the remote address of connection with bind with pipeline.
func (p *duplexPipeline) Remote() net.Addr {
	if p.conn != nil {
		return p.conn.RemoteAddr()
	}
	return &UnknownAddr{}
}

// Start only work while pipeline is in READ state. It will start three goroutine worker for
// inbound and outbound data processing and change state from READ to RUNNING.
func (p *duplexPipeline) Start() error {

	p.stateMutex.Lock()
	defer p.stateMutex.Unlock()

	if p.state != stateReady {
		// Only work while pipeline is in READY state.
		return nil
	}

	// Start handlers
	p.startConnReadHandler()
	p.startInboundHandler()
	p.startOutboundHandler()

	p.state = stateRunning
	p.stateWaitGroup.Add(1)

	return nil
}

func (p *duplexPipeline) startConnReadHandler() {
	p.connReadSyncC = make(chan uint8)
	go func() {
		p.handleConnRead()
		close(p.connReadSyncC)
	}()
}

func (p *duplexPipeline) handleConnRead() {

	Trace("ConnReadHandler for remote %s start.\n", p.conn.RemoteAddr().String())
	defer Trace("ConnReadHandler for remote %s stop.\n", p.conn.RemoteAddr().String())

	// Channel activate
	if err := p.handler.ChannelActivate(p.channel); err != nil {
		p.handler.ChannelError(p.channel, err)
	}

	// Init buffer
	readBuffer := make([]byte, readBufferSize)
	byteBuffer := NewByteBuf(byteBufferSize)

	// Read bytes from connection
	for {
		count, err := p.conn.Read(readBuffer)
		if err != nil {
			go p.Stop()
			// Channel inactivate
			if err := p.handler.ChannelInactivate(p.channel); err != nil {
				p.handler.ChannelError(p.channel, err)
			}
			return
		}

		Trace("ConnReadHandler read %d bytes from remote %s.\n", count, p.conn.RemoteAddr().String())

		byteBuffer.WriteBytes(readBuffer[:count])
		for {
			result, err := p.decoder.Decode(byteBuffer)
			if err != nil {
				p.handler.ChannelError(p.channel, err)
			} else if result != nil {
				p.inboundDataC <- result
			} else {
				break
			}
		}
		byteBuffer.Release()

	}
}

func (p *duplexPipeline) startInboundHandler() {
	p.inboundSyncC = make(chan uint8)
	go func() {
		p.handleInbound()
		close(p.inboundSyncC)
	}()
}

func (p *duplexPipeline) handleInbound() {

	Trace("InboundHandler for remote %s start.\n", p.conn.RemoteAddr().String())

	defer func() {
		Trace("InboundHandler for remote %s stop.\n", p.conn.RemoteAddr().String())
	}()

	for {
		select {
		case inboundData := <-p.inboundDataC:
			if err := p.handler.ChannelRead(p.channel, inboundData); err != nil {
				p.handler.ChannelError(p.channel, err)
			}
			continue
		case <-p.inboundHandlerStopC:
			return
		}
	}
}

func (p *duplexPipeline) startOutboundHandler() {
	p.outboundSyncC = make(chan uint8)
	go func() {
		p.handleOutbound()
		close(p.outboundSyncC)
	}()
}

func (p *duplexPipeline) handleOutbound() {

	Trace("OutboundHandler for remote %s start.", p.conn.RemoteAddr().String())

	defer func() {
		Trace("OutboundHandler for remote %s stop.", p.conn.RemoteAddr().String())
	}()

	for {
		select {
		case outboundData := <-p.outboundDataC:
			data := outboundData.Data
			callback := outboundData.Callback
			// Encode
			encodeResult, encodeErr := p.encoder.Encode(data)
			if encodeErr != nil {
				p.handler.ChannelError(p.channel, encodeErr)
				if callback != nil {
					// Invoke callback
					callback(encodeErr)
				}
				continue
			}
			// Write
			writeCount, writeErr := p.conn.Write(encodeResult)
			if callback != nil {
				// Invoke callback
				callback(writeErr)
				if writeErr == nil {
					Trace("OutboundHandler write %d bytes to remote %s.",
						writeCount, p.conn.RemoteAddr().String())
				}
				continue
			}
		case <-p.outboundHandlerStopC:
			return
		}
	}
}

// Init make pipeline init and change it's state from NEW to READY.
func (p *duplexPipeline) Init() error {

	p.stateMutex.Lock()
	defer p.stateMutex.Unlock()

	if p.state == stateNew {

		// Check conn, codec and handler
		if p.conn == nil {
			return NilConnError
		}
		if p.decoder == nil {
			return NilDecoderError
		}
		if p.encoder == nil {
			return NilEncoderError
		}
		if p.handler == nil {
			return NilHandlerError
		}

		// Init data chan.
		p.inboundDataC = make(chan interface{}, dataChanSize)
		p.outboundDataC = make(chan outboundEntity, dataChanSize)

		// Init handler command chan.
		p.inboundHandlerStopC = make(chan uint8, cmdChanSize)
		p.outboundHandlerStopC = make(chan uint8, cmdChanSize)

		// Init network channel and make it bind with current pipeline.
		p.channel = NewChannel(p)

		p.state = stateReady
	}

	return nil
}

// Stop will stop pipeline and close connection.
func (p *duplexPipeline) Stop() {

	// Mutex
	p.stateMutex.Lock()
	defer p.stateMutex.Unlock()

	if p.state != stateRunning {
		return
	}

	// Send  stop cmd to handlers
	close(p.inboundHandlerStopC)
	close(p.outboundHandlerStopC)
	// Await termination
	if p.inboundSyncC != nil {
		<-p.inboundSyncC
	}
	if p.outboundSyncC != nil {
		<-p.outboundSyncC
	}

	// Close reader and connection
	p.conn.Close()
	if p.connReadSyncC != nil {
		<-p.connReadSyncC
	}

	// Close data channels
	close(p.inboundDataC)
	close(p.outboundDataC)

	// Change state
	p.state = stateShutdown
	p.stateWaitGroup.Done()

	// Cleanup runtime objects.
	p.connReadSyncC = nil
	p.inboundSyncC = nil
	p.outboundSyncC = nil
}

// IsRunning check whether or not it is running
func (p *duplexPipeline) IsRunning() bool {
	p.stateMutex.RLock()
	defer p.stateMutex.RUnlock()
	return p.state == stateRunning
}

// Send will put message object into outbound data queue and wait until message
// have been handled by outbound handler if pipeline current running.
func (p *duplexPipeline) Send(msg interface{}) error {
	sendErrC := make(chan error, 1)
	p.SendFuture(msg, func(err error) {
		sendErrC <- err
		close(sendErrC)
	})
	return <-sendErrC
}

// SendFuture put message object into outbound data queue and register callback
// function if pipeline current running. The callback function will be invoked
// by outbound handler after data processed.
func (p *duplexPipeline) SendFuture(msg interface{}, callback func(err error)) {
	if msg == nil {
		return
	}
	p.stateMutex.RLock()
	defer p.stateMutex.RUnlock()
	if p.state != stateRunning {
		if callback != nil {
			callback(errors.New("pipeline closed"))
		}
	}
	if p.outboundDataC != nil {
		p.outboundDataC <- outboundEntity{
			Data:     msg,
			Callback: callback,
		}
	}
}

// Sync block invoker goroutine until pipeline stop.
func (p *duplexPipeline) Sync() {
	p.stateWaitGroup.Wait()
}
