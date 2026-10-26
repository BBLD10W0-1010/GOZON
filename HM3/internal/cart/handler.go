package cart

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handler struct {
	service *Service
}

func NewHandler(client ProductClient) http.Handler {
	return newHandler(NewService(client))
}

func newHandler(service *Service) http.Handler {
	h := &handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /user/{user_id}/cart/{sku_id}", h.add)
	mux.HandleFunc("GET /user/{user_id}/cart", h.get)
	mux.HandleFunc("GET /swagger.json", serveSwaggerJSON)
	mux.HandleFunc("GET /swagger/", serveSwaggerUI)
	return logging(mux)
}

func (h *handler) add(w http.ResponseWriter, r *http.Request) {
	req, err := readAddRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.service.AddToCart(r.Context(), req.userID, req.sku, req.count); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	userID, err := positiveID(r.PathValue("user_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	cart, err := h.service.GetCart(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cart)
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
