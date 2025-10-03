package wasm

import (
	"encoding/binary"
	"encoding/json"
)

type ResponseCode uint8

const (
	Success ResponseCode = iota
	WasmRuntimeError
)

type WasmHostFunctionResponse[T any] struct {
	Code    ResponseCode
	Message T
}

func (r *WasmHostFunctionResponse[T]) MarshalBinary() ([]byte, error) {
	var messageBytes []byte
	var err error

	switch v := any(r.Message).(type) {
	case string:
		messageBytes = []byte(v) // use raw string bytes, no quotes
	default:
		messageBytes, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}

	bufLen := 5 + len(messageBytes)
	buf := make([]byte, bufLen)

	binary.LittleEndian.PutUint32(buf, uint32(bufLen))
	buf[4] = byte(r.Code)
	copy(buf[5:], messageBytes)

	return buf, nil
}
