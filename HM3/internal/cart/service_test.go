package cart

import (
	"context"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAddToCartValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		userID int64
		sku    int64
		count  uint32
	}{
		{"zero user", 0, 10, 1}, {"negative user", -1, 10, 1},
		{"zero SKU", 1, 0, 1}, {"negative SKU", 1, -10, 1}, {"zero count", 1, 10, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			s := NewService(stubClient(func(context.Context, *productpb.GetProductRequest) (*productpb.Product, error) {
				calls.Add(1)
				return nil, nil
			}))
			require.ErrorIs(t, s.AddToCart(context.Background(), tc.userID, tc.sku, tc.count), ErrInvalidArgument)
			require.Zero(t, calls.Load())
		})
	}
}

func TestAddToCartProductErrors(t *testing.T) {
	t.Parallel()
	for _, code := range []codes.Code{codes.NotFound, codes.Unavailable, codes.Internal, codes.DeadlineExceeded} {
		t.Run(code.String(), func(t *testing.T) {
			t.Parallel()
			s := NewService(stubClient(func(context.Context, *productpb.GetProductRequest) (*productpb.Product, error) {
				return nil, status.Error(code, "catalog error")
			}))
			err := s.AddToCart(context.Background(), 1, 10, 1)
			require.Equal(t, code, status.Code(err))
			_, err = s.GetCart(context.Background(), 1)
			require.ErrorIs(t, err, ErrCartNotFound)
		})
	}
}

func TestAddToCartInvalidProduct(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		product *productpb.Product
	}{
		{"nil", nil}, {"wrong SKU", &productpb.Product{Sku: 99}},
		{"negative price", &productpb.Product{Sku: 10, Price: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewService(stubClient(func(context.Context, *productpb.GetProductRequest) (*productpb.Product, error) {
				return tc.product, nil
			}))
			require.Error(t, s.AddToCart(context.Background(), 1, 10, 1))
			_, err := s.GetCart(context.Background(), 1)
			require.ErrorIs(t, err, ErrCartNotFound)
		})
	}
}

func TestCartContents(t *testing.T) {
	t.Parallel()
	s := NewService(catalogClient())
	require.NoError(t, s.AddToCart(context.Background(), 1, 20, 1))
	require.NoError(t, s.AddToCart(context.Background(), 1, 10, 2))
	require.NoError(t, s.AddToCart(context.Background(), 1, 10, 3))
	require.NoError(t, s.AddToCart(context.Background(), 2, 30, 1))
	got, err := s.GetCart(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, Cart{Items: []Item{
		{SKU: 10, Name: "Book", Count: 5, Price: 10},
		{SKU: 20, Name: "Book", Count: 1, Price: 10},
	}, TotalPrice: 60}, got)
	got.Items[0].Count = 100
	again, err := s.GetCart(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, uint32(5), again.Items[0].Count)
}

func TestCartOverflow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		price int32
		count uint32
	}{
		{"count", 0, math.MaxUint32},
		{"total", math.MaxInt32, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewService(stubClient(func(_ context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
				return &productpb.Product{Sku: req.GetSku(), Price: tc.price}, nil
			}))
			require.NoError(t, s.AddToCart(context.Background(), 1, 10, tc.count))
			require.ErrorIs(t, s.AddToCart(context.Background(), 1, 10, 1), ErrOverflow)
			got, err := s.GetCart(context.Background(), 1)
			require.NoError(t, err)
			require.Equal(t, tc.count, got.Items[0].Count)
		})
	}
}

func TestAddToCartRepricing(t *testing.T) {
	t.Parallel()
	s := seedCart(t, 10)
	s.client = pricedClient(math.MaxInt32)
	require.NoError(t, s.AddToCart(t.Context(), 1, 10, 1))
	require.ErrorIs(t, s.AddToCart(t.Context(), 1, 20, 1), ErrOverflow)
	got, err := s.GetCart(t.Context(), 1)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, uint32(2), got.Items[0].Count)
	require.Equal(t, uint32(math.MaxUint32-1), got.TotalPrice)
}

func TestGetCartWithoutProductRequests(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		userID int64
		err    error
	}{
		{"empty", 1, ErrCartNotFound}, {"zero user", 0, ErrInvalidArgument},
		{"negative user", -1, ErrInvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			s := NewService(stubClient(func(context.Context, *productpb.GetProductRequest) (*productpb.Product, error) {
				calls.Add(1)
				return nil, nil
			}))
			got, err := s.GetCart(context.Background(), tc.userID)
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, Cart{}, got)
			require.Zero(t, calls.Load())
		})
	}
}

func TestGetCartParallelRequests(t *testing.T) {
	t.Parallel()
	s := seedCart(t, 30, 10, 20)
	client, deadlines := parallelClient(3)
	s.client = client
	s.timeout = time.Second
	got, err := s.GetCart(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got.Items, 3)
	require.Len(t, deadlines, 3)
	first := <-deadlines
	require.False(t, first.IsZero())
	require.Equal(t, first, <-deadlines)
	require.Equal(t, first, <-deadlines)
}

func TestGetCartCommonTimeout(t *testing.T) {
	t.Parallel()
	s := seedCart(t, 10, 20)
	s.client = stubClient(func(ctx context.Context, _ *productpb.GetProductRequest) (*productpb.Product, error) {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	})
	s.timeout = 20 * time.Millisecond
	got, err := s.GetCart(context.Background(), 1)
	require.Equal(t, codes.DeadlineExceeded, status.Code(err))
	require.Equal(t, Cart{}, got)
}

func TestAddToCartHasDeadline(t *testing.T) {
	t.Parallel()
	deadlines := make(chan time.Time, 1)
	s := NewService(stubClient(func(ctx context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
		deadline, _ := ctx.Deadline()
		deadlines <- deadline
		return &productpb.Product{Sku: req.GetSku()}, nil
	}))
	require.NoError(t, s.AddToCart(context.Background(), 1, 10, 1))
	require.False(t, (<-deadlines).IsZero())
}

func TestGetCartRefreshesProducts(t *testing.T) {
	t.Parallel()
	s := NewService(catalogClient())
	require.NoError(t, s.AddToCart(context.Background(), 1, 10, 2))
	s.client = pricedClient(15)
	got, err := s.GetCart(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, uint32(15), got.Items[0].Price)
	require.Equal(t, uint32(30), got.TotalPrice)
}

func TestGetCartDisappearedProduct(t *testing.T) {
	t.Parallel()
	s := seedCart(t, 10, 20)
	s.client = stubClient(func(_ context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
		if req.GetSku() == 20 {
			return nil, status.Error(codes.NotFound, "deleted")
		}
		return &productpb.Product{Sku: req.GetSku(), Price: 10}, nil
	})
	got, err := s.GetCart(context.Background(), 1)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Equal(t, Cart{}, got)
}

func TestGetCartPriceOverflow(t *testing.T) {
	t.Parallel()
	s := NewService(catalogClient())
	require.NoError(t, s.AddToCart(context.Background(), 1, 10, 3))
	s.client = pricedClient(math.MaxInt32)
	got, err := s.GetCart(context.Background(), 1)
	require.ErrorIs(t, err, ErrOverflow)
	require.Equal(t, Cart{}, got)
}

func TestAddToCartCanceled(t *testing.T) {
	t.Parallel()
	s := NewService(stubClient(func(ctx context.Context, _ *productpb.GetProductRequest) (*productpb.Product, error) {
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	}))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := s.AddToCart(ctx, 1, 10, 1)
	require.Equal(t, codes.Canceled, status.Code(err))
	_, err = s.GetCart(context.Background(), 1)
	require.ErrorIs(t, err, ErrCartNotFound)
}
