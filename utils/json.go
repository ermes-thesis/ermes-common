package utils

import (
	"bytes"
	"encoding/json"
)

// MarshalJSONUnescaped https://stackoverflow.com/a/69502657
func MarshalJSONUnescaped(i any) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(i)
	return bytes.TrimRight(buffer.Bytes(), "\n"), err
}
