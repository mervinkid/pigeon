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
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	TagSize    = 1
	LengthSize = 4
)

// TLVConfig is a data struct provide configuration properties for both
// tlvFrameDecoder and tlvFrameEncoder.
//  +----------+-----------+-----------+
//  |    TAG   |  LENGTH   |   VALUE   |
//  | (1 byte) | (4 bytes) | (payload) |
//  +----------+-----------+-----------+
//       ↑
//    TagValue
//
type TLVConfig struct {
	TagValue   uint8
	FrameLimit uint32
}

// tlvFrameDecoder is a bytes to bytes decoder implementation of FrameDecoder with TLV format.
//  +----------+-----------+-----------+
//  |    TAG   |  LENGTH   |   VALUE   |
//  | (1 byte) | (4 bytes) | (payload) |
//  +----------+-----------+-----------+
//       ↑
//    TagValue
//
// Notes:
//  Decode []byte → []byte.
type tlvFrameDecoder struct {
	Config TLVConfig
	// Decode buffer
	hasTag      bool
	hasLength   bool
	tagValue    uint8
	lengthValue uint32
}

func (c *tlvFrameDecoder) Decode(in ByteBuf) (interface{}, error) {

	// Parse T(tag)
	if !c.hasTag {
		if in.ReadableBytes() < TagSize {
			// No enough bytes to parse.
			return c.decodeNothing()
		}
		tmpBytes := in.ReadBytes(TagSize)
		reader := bytes.NewReader(tmpBytes)
		var tag uint8
		err := binary.Read(reader, binary.BigEndian, &tag)
		if err != nil {
			return c.decodeFailure(err.Error())
		}
		if tag != c.Config.TagValue {
			return c.decodeFailure("illegal tag found")
		}
		c.tagValue = tag
		c.hasTag = true
	}

	// Parse L(length)
	if c.hasTag && !c.hasLength {
		if in.ReadableBytes() < LengthSize {
			// No enough bytes to parse.
			return nil, nil
		}
		tmpBytes := in.ReadBytes(LengthSize)
		reader := bytes.NewReader(tmpBytes)
		var length uint32
		err := binary.Read(reader, binary.BigEndian, &length)
		if err != nil {
			return c.decodeFailure(err.Error())
		}
		c.lengthValue = length
		c.hasLength = true
	}

	// Parse V(value)
	if c.hasTag && c.hasLength {
		if in.ReadableBytes() < int(c.lengthValue) {
			// No enough bytes to parse.
			return nil, nil
		}
		tmpBytes := in.ReadBytes(int(c.lengthValue))
		// Validate frame size
		if c.Config.FrameLimit > 0 && uint64(TagSize+LengthSize)+uint64(len(tmpBytes)) > uint64(c.Config.FrameLimit) {
			return c.decodeFailure("frame size larger than limit")
		}
		return c.decodeSuccess(tmpBytes)
	}

	return c.decodeNothing()
}

// resetBuffer reset all buffer data inside tlvFrameDecoder.
func (c *tlvFrameDecoder) resetBuffer() {
	c.hasTag = false
	c.hasLength = false
	c.tagValue = 0
	c.lengthValue = 0
}

func (c *tlvFrameDecoder) decodeNothing() (interface{}, error) {
	return c.decodeSuccess(nil)
}

func (c *tlvFrameDecoder) decodeSuccess(result interface{}) (interface{}, error) {
	if result != nil {
		c.resetBuffer()
	}
	return result, nil
}

func (c *tlvFrameDecoder) decodeFailure(cause string) (interface{}, error) {
	return nil, NewDecodeError("tlvFrameDecoder", cause)
}

// NewTLVFrameDecoder create instance of tlvFrameDecoder with specified configuration.
func NewTLVFrameDecoder(config TLVConfig) FrameDecoder {
	return &tlvFrameDecoder{Config: config}
}

// tlvFrameEncoder is a bytes to bytes encoder implementation of FrameEncoder with TLV format.
//  +----------+-----------+-----------+
//  |    TAG   |  LENGTH   |   VALUE   |
//  | (1 byte) | (4 bytes) | (payload) |
//  +----------+-----------+-----------+
//       ↑
//    TagValue
//
// Notes:
//  Encode []byte → []byte.
type tlvFrameEncoder struct {
	Config TLVConfig
}

func (c *tlvFrameEncoder) Encode(msg interface{}) ([]byte, error) {

	// Inbound type must be []byte
	payload, payloadTransform := msg.([]byte)
	if !payloadTransform {
		return c.encodeFailure("can not transform input to []byte")
	}

	payloadLength := uint32(len(payload))

	// Validate frame size
	frameSize := uint64(payloadLength + LengthSize + TagSize)
	if c.Config.FrameLimit > 0 && frameSize > uint64(c.Config.FrameLimit) {
		cause := fmt.Sprintf("frame size %d larger than limit %d", frameSize, c.Config.FrameLimit)
		return c.encodeFailure(cause)
	}

	// Assemble
	frameByteBuf := NewByteBuf(int(frameSize))
	binary.Write(frameByteBuf, binary.BigEndian, c.Config.TagValue)
	binary.Write(frameByteBuf, binary.BigEndian, payloadLength)
	frameByteBuf.WriteBytes(payload)

	// Validate result
	if frameSize != uint64(frameByteBuf.ReadableBytes()) {
		cause := fmt.Sprintf("ByteBuf issue")
		return c.encodeFailure(cause)
	}

	result := frameByteBuf.ReadBytes(frameByteBuf.ReadableBytes())

	return c.encodeSuccess(result)
}

func (c *tlvFrameEncoder) encodeSuccess(result []byte) ([]byte, error) {
	return result, nil
}

func (c *tlvFrameEncoder) encodeFailure(cause string) ([]byte, error) {
	return nil, NewEncodeError("tlvFrameEncoder", cause)
}

// NewTLVFrameEncoder create instance of tlvFrameEncoder with specified configuration.
func NewTLVFrameEncoder(config TLVConfig) FrameEncoder {
	return &tlvFrameEncoder{Config: config}
}
