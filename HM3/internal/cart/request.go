package cart

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

const maxBodySize = 1024

type addRequest struct {
	userID int64
	sku    int64
	count  uint32
}

func readAddRequest(r *http.Request) (addRequest, error) {
	userID, err := positiveID(r.PathValue("user_id"))
	if err != nil {
		return addRequest{}, err
	}
	sku, err := positiveID(r.PathValue("sku_id"))
	if err != nil {
		return addRequest{}, err
	}
	count, err := readCount(r)
	if err != nil {
		return addRequest{}, err
	}
	return addRequest{userID: userID, sku: sku, count: count}, nil
}

func positiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrInvalidArgument
	}
	return id, nil
}

func readCount(r *http.Request) (uint32, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
	if err != nil || len(data) > maxBodySize || !json.Valid(data) {
		return 0, ErrInvalidArgument
	}
	value, err := countValue(bytes.TrimSpace(data))
	if err != nil {
		return 0, err
	}
	count, err := strconv.ParseUint(string(value), 10, 32)
	if err != nil || count == 0 {
		return 0, ErrInvalidArgument
	}
	return uint32(count), nil
}

func countValue(data []byte) ([]byte, error) {
	if data[0] != '{' {
		return data, nil
	}
	var body struct {
		Count json.RawMessage `json:"count"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return nil, ErrInvalidArgument
	}
	return body.Count, nil
}
