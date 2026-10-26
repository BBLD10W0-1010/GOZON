package cart

import (
	_ "embed"
	"net/http"
)

//go:embed swagger.json
var swaggerJSON []byte

//go:embed swagger.html
var swaggerHTML []byte

func serveSwaggerJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(swaggerJSON)
}

func serveSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(swaggerHTML)
}
