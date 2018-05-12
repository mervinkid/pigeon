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

	"github.com/vmihailenco/msgpack"
)

type ApolloEntity interface {
	TypeCode() uint16
}

type ApolloConfig struct {
	TLVConfig
	entityConstructors map[uint16]func() ApolloEntity
}

func (c *ApolloConfig) RegisterEntity(constructor func() ApolloEntity) {
	c.initConfig()
	if constructor != nil {
		if testEntity := constructor(); testEntity != nil {
			c.entityConstructors[testEntity.TypeCode()] = constructor
		}
	}
}

func (c *ApolloConfig) createEntity(typeCode uint16) ApolloEntity {
	c.initConfig()
	if constructor := c.entityConstructors[typeCode]; constructor != nil {
		return constructor()
	}
	return nil
}

func (c *ApolloConfig) initConfig() {
	if c.entityConstructors == nil {
		c.entityConstructors = make(map[uint16]func() ApolloEntity)
	}
}

// apolloFrameDecoder is a bytes to ApolloEntity decode implementation of FrameDecode based on tlvFrameDecoder
// using MessagePack for payload data deserialization.
//  +----------+-----------+---------------------------+
//  |    TAG   |  LENGTH   |           VALUE           |
//  | (1 byte) | (4 bytes) |   2 bytes   | serialized  |
//  |          |           |  type code  |    data     |
//  +----------+-----------+---------------------------+
// Decode:
//  []byte → ApolloEntity(*pointer)
type apolloFrameDecoder struct {
	Config     ApolloConfig
	tlvDecoder FrameDecoder
}

func (d *apolloFrameDecoder) Decode(in ByteBuf) (interface{}, error) {

	if in.ReadableBytes() == 0 {
		return d.decodeNothing()
	}

	// Decode inbound with tlvFrameDecoder
	d.initTLVDecoder()
	tlvPayload, tlvErr := d.tlvDecoder.Decode(in)
	if tlvPayload == nil && tlvErr == nil {
		return d.decodeNothing()
	}
	if tlvErr != nil {
		return d.decodeFailure(tlvErr.Error())
	}

	// Init ByteBuf for MessagePack deserialization.
	tlvPayloadByteBuffer := NewByteBuf(len(tlvPayload.([]byte)))
	tlvPayloadByteBuffer.WriteBytes(tlvPayload.([]byte))

	// Parse 2 bytes of message type code.
	if tlvPayloadByteBuffer.ReadableBytes() < 2 {
		return d.decodeFailure("illegal payload")
	}
	var typeCode uint16
	binary.Read(tlvPayloadByteBuffer, binary.BigEndian, &typeCode)

	// Parse reset bytes for serialized data.
	serializedBytes := tlvPayloadByteBuffer.ReadBytes(tlvPayloadByteBuffer.ReadableBytes())
	if entity := d.Config.createEntity(typeCode); entity != nil {
		if unmarshalErr := msgpack.Unmarshal(serializedBytes, entity); unmarshalErr != nil {
			return d.decodeFailure(unmarshalErr.Error())
		} else {
			return d.decodeSuccess(entity)
		}
	}
	return d.decodeNothing()
}

func (d *apolloFrameDecoder) initTLVDecoder() {
	if d.tlvDecoder == nil {
		d.tlvDecoder = NewTLVFrameDecoder(d.Config.TLVConfig)
	}
}

func (d *apolloFrameDecoder) decodeNothing() (interface{}, error) {
	return d.decodeSuccess(nil)
}

func (d *apolloFrameDecoder) decodeSuccess(result interface{}) (interface{}, error) {
	return result, nil
}

func (d *apolloFrameDecoder) decodeFailure(cause string) (interface{}, error) {
	return nil, NewDecodeError("apolloFrameDecoder", cause)
}

// NewApolloFrameDecoder create a new apolloFrameDecoder instance with configuration.
func NewApolloFrameDecoder(config ApolloConfig) FrameDecoder {
	return &apolloFrameDecoder{Config: config}
}

// apolloFrameEncoder is a ApolloEntity to bytes encoder implementation of FrameEncode based on tlvFrameEncoder
// using MessagePack for payload data serialization.
//  +----------+-----------+---------------------------+
//  |    TAG   |  LENGTH   |           VALUE           |
//  | (1 byte) | (4 bytes) |   2 bytes   | serialized  |
//  |          |           |  type code  |    data     |
//  +----------+-----------+---------------------------+
// Encode:
//  ApolloEntity(*pointer) → []byte
type apolloFrameEncoder struct {
	Config     ApolloConfig
	tlvEncoder FrameEncoder
}

func (e *apolloFrameEncoder) Encode(msg interface{}) ([]byte, error) {

	// Message must be an implementation of ApolloEntity interface.
	var entity ApolloEntity
	switch message := msg.(type) {
	case ApolloEntity:
		entity = message
	default:
		return e.encodeFailure("message is not valid implementation of ApolloEntity interface")
	}

	// Marshal entity to bytes.
	typeCode := entity.TypeCode()
	marshaledBytes, marshalErr := msgpack.Marshal(entity)
	if marshalErr != nil {
		return e.encodeFailure(marshalErr.Error())
	}
	// Build frame payload with marshaled bytes and type code.
	payloadByteBuffer := NewByteBuf(2 + len(marshaledBytes))
	binary.Write(payloadByteBuffer, binary.BigEndian, typeCode)
	binary.Write(payloadByteBuffer, binary.BigEndian, marshaledBytes)

	// Encode with TLVEncoder
	e.initTLVEncoder()
	frameBytes, encodeErr := e.tlvEncoder.Encode(payloadByteBuffer.ReadBytes(payloadByteBuffer.ReadableBytes()))
	if encodeErr != nil {
		return e.encodeFailure(encodeErr.Error())
	}

	return e.encodeSuccess(frameBytes)
}

func (e *apolloFrameEncoder) initTLVEncoder() {
	if e.tlvEncoder == nil {
		e.tlvEncoder = NewTLVFrameEncoder(e.Config.TLVConfig)
	}
}

func (e *apolloFrameEncoder) encodeSuccess(result []byte) ([]byte, error) {
	return result, nil
}

func (e *apolloFrameEncoder) encodeFailure(cause string) ([]byte, error) {
	return nil, NewEncodeError("apolloFrameEncoder", cause)
}

// NewApolloFrameEncoder create a new apolloFrameEncoder instance with configuration.
func NewApolloFrameEncoder(config ApolloConfig) FrameEncoder {
	return &apolloFrameEncoder{Config: config}
}
