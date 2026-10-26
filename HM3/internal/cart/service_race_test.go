//go:build race

package cart

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConcurrentCartRequests(t *testing.T) {
	t.Parallel()
	const workers, additions = 16, 20
	h := NewHandler(catalogClient())
	initial := httptest.NewRecorder()
	h.ServeHTTP(initial, httptest.NewRequest(http.MethodPost, "/user/1/cart/10", strings.NewReader(`{"count":1}`)))
	require.Equal(t, http.StatusOK, initial.Code)
	for _, err := range runCartWorkers(h, workers, additions) {
		require.NoError(t, err)
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/user/1/cart", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var got Cart
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &got))
	require.Len(t, got.Items, 1)
	require.Equal(t, uint32(1+workers*additions), got.Items[0].Count)
	require.Equal(t, uint32(10*(1+workers*additions)), got.TotalPrice)
}

func runCartWorkers(h http.Handler, workers, additions int) []error {
	start := make(chan struct{})
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func() {
			defer wg.Done()
			<-start
			errs[i] = cartWorkload(h, additions)
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func cartWorkload(h http.Handler, additions int) error {
	for range additions {
		requests := []*http.Request{
			httptest.NewRequest(http.MethodPost, "/user/1/cart/10", strings.NewReader(`{"count":1}`)),
			httptest.NewRequest(http.MethodGet, "/user/1/cart", nil),
		}
		for _, req := range requests {
			response := httptest.NewRecorder()
			h.ServeHTTP(response, req)
			if response.Code != http.StatusOK {
				return fmt.Errorf("%s returned %d", req.Method, response.Code)
			}
		}
	}
	return nil
}
