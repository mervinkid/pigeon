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
	"time"
)

type TCPConfig struct {
	Port            int
	IP              net.IP
	KeepAlive       bool
	KeepAlivePeriod time.Duration
}

// ServerConfig provide properties for server configuration
type ServerConfig struct {
	TCPConfig
	AcceptorSize uint8
}

// ClientConfig provide properties for client configuration
type ClientConfig struct {
	TCPConfig
	Timeout time.Duration
}

// TryApplyTCPConfig will setup specified tcp connection with specified config if possible.
func (c *TCPConfig) ApplyTCP(conn *net.TCPConn) {
	if conn != nil {
		conn.SetKeepAlive(c.KeepAlive)
		conn.SetKeepAlivePeriod(c.KeepAlivePeriod)
	}
}
