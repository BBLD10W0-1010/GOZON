package cart

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatusWriterImplicitHeader(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	w := &statusWriter{ResponseWriter: response}
	_, err := w.Write([]byte("ok"))
	require.NoError(t, err)
	w.WriteHeader(http.StatusInternalServerError)
	require.Equal(t, http.StatusOK, w.status)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "ok", response.Body.String())
}

func TestStatusWriterFirstHeader(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	w := &statusWriter{ResponseWriter: response}
	w.WriteHeader(http.StatusCreated)
	w.WriteHeader(http.StatusInternalServerError)
	require.Equal(t, http.StatusCreated, w.status)
	require.Equal(t, http.StatusCreated, response.Code)
}
