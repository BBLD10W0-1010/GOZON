package cart

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"gozon/internal/productpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProductClient interface {
	GetProduct(context.Context, *productpb.GetProductRequest, ...grpc.CallOption) (*productpb.Product, error)
}

type Item struct {
	SKU   int64  `json:"sku"`
	Name  string `json:"name"`
	Count uint32 `json:"count"`
	Price uint32 `json:"price"`
}

type cartData struct {
	items map[int64]Item
	total uint32
}

type Handler struct {
	client ProductClient
	mu     sync.RWMutex
	carts  map[int64]*cartData
}

func NewHandler(client ProductClient) http.Handler {
	h := &Handler{client: client, carts: make(map[int64]*cartData)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /user/{user_id}/cart/{sku_id}", h.add)
	mux.HandleFunc("GET /user/{user_id}/cart", h.get)
	mux.HandleFunc("GET /swagger.json", serveSwaggerJSON)
	mux.HandleFunc("GET /swagger/", serveSwaggerUI)
	return logging(mux)
}

func positiveID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("identifier must be a positive integer")
	}
	return id, nil
}

func readCount(r *http.Request) (uint32, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 1025))
	if err != nil || len(data) > 1024 {
		return 0, errors.New("invalid request body")
	}
	data = bytes.TrimSpace(data)
	var number json.Number
	if len(data) > 0 && data[0] == '{' {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var body struct {
			Count json.Number `json:"count"`
		}
		if err := decoder.Decode(&body); err != nil {
			return 0, err
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return 0, errors.New("multiple JSON values")
		}
		number = body.Count
	} else if err := json.Unmarshal(data, &number); err != nil {
		return 0, err
	}
	count, err := strconv.ParseUint(string(number), 10, 32)
	if err != nil || count == 0 {
		return 0, errors.New("count must be a positive uint32")
	}
	return uint32(count), nil
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	userID, err := positiveID(r.PathValue("user_id"))
	if err != nil {
		http.Error(w, "user_id must be a positive integer", http.StatusBadRequest)
		return
	}
	sku, err := positiveID(r.PathValue("sku_id"))
	if err != nil {
		http.Error(w, "sku_id must be a positive integer", http.StatusBadRequest)
		return
	}
	count, err := readCount(r)
	if err != nil {
		http.Error(w, "count must be a positive uint32", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	product, err := h.client.GetProduct(ctx, &productpb.GetProductRequest{Sku: sku})
	if status.Code(err) == codes.NotFound {
		http.Error(w, "product does not exist", http.StatusPreconditionFailed)
		return
	}
	if err != nil || product == nil || product.GetSku() != sku || product.GetPrice() < 0 {
		log.Printf("product lookup failed for sku %d: product=%v, error=%v", sku, product, err)
		http.Error(w, "product service unavailable", http.StatusInternalServerError)
		return
	}
	price := uint32(product.GetPrice())
	h.mu.Lock()
	defer h.mu.Unlock()
	c := h.carts[userID]
	if c == nil {
		c = &cartData{items: make(map[int64]Item)}
	}
	previous := c.items[sku]
	name := product.GetName()
	if previous.Count > 0 {
		// Existing cart lines keep the price and name recorded at first addition.
		price, name = previous.Price, previous.Name
	}
	if uint64(previous.Count)+uint64(count) > math.MaxUint32 || uint64(price)*uint64(count) > math.MaxUint32 || uint64(c.total)+uint64(price)*uint64(count) > math.MaxUint32 {
		http.Error(w, "cart value exceeds uint32", http.StatusInternalServerError)
		return
	}
	c.items[sku] = Item{SKU: sku, Name: name, Count: previous.Count + count, Price: price}
	c.total += price * count
	h.carts[userID] = c
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, err := positiveID(r.PathValue("user_id"))
	if err != nil {
		http.Error(w, "user_id must be a positive integer", http.StatusBadRequest)
		return
	}
	h.mu.RLock()
	c := h.carts[userID]
	if c == nil || len(c.items) == 0 {
		h.mu.RUnlock()
		http.Error(w, "cart not found", http.StatusNotFound)
		return
	}
	items := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		items = append(items, item)
	}
	total := c.total
	h.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].SKU < items[j].SKU })
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Items      []Item `json:"items"`
		TotalPrice uint32 `json:"total_price"`
	}{Items: items, TotalPrice: total})
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, wrapped.status, time.Since(start))
	})
}
