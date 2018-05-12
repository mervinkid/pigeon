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
	"encoding/binary"
)

// stringFrameDecoder is a bytes to string decoder implementation of FrameDecoder interface
// that transform inbound data from []byte to string.
//
// Example:
//  +-----------------------------------------------------------+            +----------------+
//  |0x48|0x65|0x6c|0x6c|0x6f|0x20|0x57|0x6f|0x72|0x6c|0x64|0x2e| → decode → | "Hello World." |
//  +-----------------------------------------------------------+            +----------------+
type stringFrameDecoder struct {
}

func (d *stringFrameDecoder) Decode(in ByteBuf) (interface{}, error) {

	if in.ReadableBytes() == 0 {
		return d.decodeNothing()
	}
	var result string
	err := binary.Read(in, binary.BigEndian, &result)
	if err != nil {
		return d.decodeFailure(err.Error())
	}
	return d.decodeSuccess(result)
}

func (d *stringFrameDecoder) decodeNothing() (interface{}, error) {
	return d.decodeSuccess(nil)
}

func (d *stringFrameDecoder) decodeSuccess(result interface{}) (interface{}, error) {
	return result, nil
}

func (d *stringFrameDecoder) decodeFailure(cause string) (interface{}, error) {
	return nil, NewDecodeError("stringFrameDecoder", cause)
}

// NewStringFrameDecoder create a new stringFrameDecoder instance.
func NewStringFrameDecoder() FrameDecoder {
	return &stringFrameDecoder{}
}

// stringFrameEncoder is a string to bytes encoder implementation of FrameEncoder interface
// that transform outbound data from string to []byte.
//
// Example:
//  +----------------+            +-----------------------------------------------------------+
//  | "Hello World." | → encode → |0x48|0x65|0x6c|0x6c|0x6f|0x20|0x57|0x6f|0x72|0x6c|0x64|0x2e|
//  +----------------+            +-----------------------------------------------------------+
type stringFrameEncoder struct {
}

func (e *stringFrameEncoder) Encode(msg interface{}) ([]byte, error) {

	if msg == nil {
		return e.encodeSuccess([]byte{})
	}

	// Check inbound type.
	payload, payloadTransform := msg.(string)
	if !payloadTransform {
		return e.encodeFailure("can not transform input to string")
	}

	return e.encodeSuccess([]byte(payload))
}

func (e *stringFrameEncoder) encodeSuccess(result []byte) ([]byte, error) {
	return result, nil
}

func (e *stringFrameEncoder) encodeFailure(cause string) ([]byte, error) {
	return nil, NewEncodeError("stringFrameEncoder", cause)
}

// NewStringFrameEncoder create a new stringFrameEncoder instance.
func NewStringFrameEncoder() FrameEncoder {
	return &stringFrameEncoder{}
}
