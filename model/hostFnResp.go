package model

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

type ResponseCode uint8

const (
	Success ResponseCode = iota
	WasmRuntimeError
	Unauthorized
	NotFound
	RedisError
	GroupTokenProviderError
	HttpError
)

func (c ResponseCode) String() string {
	switch c {
	case Success:
		return "SUCCESS"
	case WasmRuntimeError:
		return "UNAUTHORIZED"
	case Unauthorized:
		return "NOT_FOUND"
	case NotFound:
		return "REDIS_ERROR"
	case RedisError:
		return "WASM_RUNTIME_ERROR"
	case HttpError:
		return "HTTP_ERROR"
	case GroupTokenProviderError:
		return "CENTRAL_REPO_ERROR"
	default:
		return "UNKNOWN"
	}
}

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

func (r *WasmHostFunctionResponse[T]) UnmarshalBinary(buf []byte) error {
	if len(buf) == 0 {
		return errors.New("response buffer is empty")
	}

	r.Code = ResponseCode(buf[0])
	if len(buf) == 1 {
		var empty T
		r.Message = empty
		return nil
	}

	data := buf[1:]

	var zero T
	switch any(zero).(type) {
	case string:
		r.Message = any(string(data)).(T)
	default:
		var parsed T
		if err := json.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("failed to unmarshal JSON message: %w", err)
		}
		r.Message = parsed
	}

	return nil
}
