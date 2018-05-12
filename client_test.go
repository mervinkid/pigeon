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

package pigeon_test

import (
	"fmt"
	"net"
	"testing"
	"time"
	"github.com/mervinkid/pigeon"
)

// Message definitions
type tCommand struct {
	Id   int
	Name string
}

func (t *tCommand) TypeCode() uint16 {
	return 1
}

func (t *tCommand) String() string {
	return fmt.Sprintf("_tCommand{Id:%d}", t.Id)
}

type _tAck struct {
	Id int
}

func (t *_tAck) TypeCode() uint16 {
	return 2
}

func (t *_tAck) String() string {
	return fmt.Sprintf("_tAck{Id:%d}", t.Id)
}

func TestClient(t *testing.T) {

	// Client config
	clientConfig := pigeon.ClientConfig{}
	clientConfig.KeepAlive = false
	clientConfig.IP = net.ParseIP("127.0.0.1")
	clientConfig.Port = 9090

	// Init client
	client := pigeon.NewClient(clientConfig, initInitializer())
	client.Start()

	go func() {
		for i := 0; i < 10; i++ {
			msg := new(tCommand)
			msg.Id = 12345 + i
			msg.Name = fmt.Sprint("TestCommand-", i)
			client.Send(msg)
			time.Sleep(1 * time.Second)
		}
	}()

	client.Stop()
}

func initInitializer() pigeon.PipelineInitializer {

	apolloConfig := initApolloConfig()
	initializer := pigeon.FunctionalPipelineInitializer{}

	// Setup decoder init function
	initializer.DecoderInit = func() pigeon.FrameDecoder {
		return pigeon.NewApolloFrameDecoder(apolloConfig)
	}

	// Setup encoder init function
	initializer.EncoderInit = func() pigeon.FrameEncoder {
		return pigeon.NewApolloFrameEncoder(apolloConfig)
	}

	// Setup handler init function
	initializer.HandlerInit = func() pigeon.ChannelHandler {
		return initHandler()
	}

	return &initializer
}

func initApolloConfig() pigeon.ApolloConfig {
	apolloConfig := pigeon.ApolloConfig{}
	// Register _tCommand
	apolloConfig.RegisterEntity(func() pigeon.ApolloEntity {
		return new(tCommand)
	})
	// Register _tAck
	apolloConfig.RegisterEntity(func() pigeon.ApolloEntity {
		return new(_tAck)
	})
	return apolloConfig
}

func initHandler() pigeon.ChannelHandler {
	handler := pigeon.FunctionalChannelHandler{}

	handler.HandleActivate = func(channel pigeon.Channel) error {
		pigeon.Info(">>> Remote ", channel.Remote().String(), " activate.")
		return nil
	}

	handler.HandleRead = func(channel pigeon.Channel, in interface{}) error {
		pigeon.Info(">>> Remote ", channel.Remote().String(), ": ", in)
		switch msg := in.(type) {
		case *tCommand:
			mId := msg.Id
			ack := &_tAck{Id: mId}
			channel.Send(ack)
			pigeon.Info(">>> Send ", ack)
		}
		return nil
	}

	handler.HandleInactivate = func(channel pigeon.Channel) error {
		pigeon.Info(">>> Remote ", channel.Remote().String(), " inactivate.")
		return nil
	}

	handler.HandleError = func(channel pigeon.Channel, err error) {
		pigeon.Error(">>> Remote ", channel.Remote().String(), " error: ", err)
	}
	return &handler
}
