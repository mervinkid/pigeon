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

type DecoderInitFunc func() FrameDecoder
type EncoderInitFunc func() FrameEncoder
type HandlerInitFunc func() ChannelHandler

// ChannelHandler is the interface provide necessary methods for pipeline initialization which invoked by pipeline.
type PipelineInitializer interface {
	//  InitDecoder used for decoder initialization.
	InitDecoder() FrameDecoder

	//  InitEncoder used for encoder initialization.
	InitEncoder() FrameEncoder

	// InitHandler used for channel handler initialization.
	InitHandler() ChannelHandler
}

// FunctionalPipelineInitializer is a public implementation of PipelineInitializer interface which
// support functional definition for pipeline initialization logic.
type FunctionalPipelineInitializer struct {
	DecoderInit DecoderInitFunc
	EncoderInit EncoderInitFunc
	HandlerInit HandlerInitFunc
}

func (i *FunctionalPipelineInitializer) InitDecoder() FrameDecoder {
	if i.DecoderInit != nil {
		return i.DecoderInit()
	}
	return nil
}

func (i *FunctionalPipelineInitializer) InitEncoder() FrameEncoder {
	if i.EncoderInit != nil {
		return i.EncoderInit()
	}
	return nil
}

func (i *FunctionalPipelineInitializer) InitHandler() ChannelHandler {
	if i.HandlerInit != nil {
		return i.HandlerInit()
	}
	return nil
}
