package cart

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewHandler(client ProductClient) http.Handler {
	return newHandler(NewService(client))
}

func newHandler(service *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /user/{user_id}/cart/{sku_id}", func(w http.ResponseWriter, r *http.Request) {
		userID, err := positiveID(r.PathValue("user_id"))
		if err != nil {
			writeError(w, err)
			return
		}
		sku, err := positiveID(r.PathValue("sku_id"))
		if err != nil {
			writeError(w, err)
			return
		}
		count, err := readCount(r)
		if err != nil {
			writeError(w, err)
			return
		}
		if err := service.AddToCart(r.Context(), userID, sku, count); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /user/{user_id}/cart", func(w http.ResponseWriter, r *http.Request) {
		userID, err := positiveID(r.PathValue("user_id"))
		if err != nil {
			writeError(w, err)
			return
		}
		cart, err := service.GetCart(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cart)
	})
	mux.HandleFunc("GET /swagger.json", serveSwaggerJSON)
	mux.HandleFunc("GET /swagger/", serveSwaggerUI)
	return logging(mux)
}

func writeError(w http.ResponseWriter, err error) {
	code, message := http.StatusInternalServerError, "internal server error"
	switch {
	case errors.Is(err, ErrInvalidArgument):
		code, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, ErrCartNotFound):
		code, message = http.StatusNotFound, err.Error()
	case status.Code(err) == codes.NotFound:
		code, message = http.StatusPreconditionFailed, "product does not exist"
	default:
		log.Printf("cart request failed: %v", err)
	}
	http.Error(w, message, code)
}

func positiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrInvalidArgument
	}
	return id, nil
}

func readCount(r *http.Request) (uint32, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 1025))
	if err != nil || len(data) > 1024 || !json.Valid(data) {
		return 0, ErrInvalidArgument
	}
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '{' {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var body struct {
			Count json.RawMessage `json:"count"`
		}
		if err := decoder.Decode(&body); err != nil {
			return 0, ErrInvalidArgument
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return 0, ErrInvalidArgument
		}
		data = body.Count
	}
	count, err := strconv.ParseUint(string(data), 10, 32)
	if err != nil || count == 0 {
		return 0, ErrInvalidArgument
	}
	return uint32(count), nil
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func logging(next http.Handler) http.Handler {
	var requests atomic.Uint64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requests.Add(1)
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		log.Printf("request_id=%d %s %s %d %s", id, r.Method, r.URL.Path, wrapped.status, time.Since(start))
	})
}
