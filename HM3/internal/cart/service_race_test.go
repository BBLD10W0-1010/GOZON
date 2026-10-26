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
	start := make(chan struct{})
	errs := make(chan error, workers*additions*2)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			for range additions {
				post := httptest.NewRecorder()
				h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/user/1/cart/10", strings.NewReader(`{"count":1}`)))
				if post.Code != http.StatusOK {
					errs <- fmt.Errorf("POST returned %d", post.Code)
				}
				get := httptest.NewRecorder()
				h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/user/1/cart", nil))
				if get.Code != http.StatusOK {
					errs <- fmt.Errorf("GET returned %d", get.Code)
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
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
