package cart

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"gozon/internal/productpb"

	"google.golang.org/grpc"
)

var (
	ErrInvalidArgument = errors.New("identifiers and count must be positive")
	ErrCartNotFound    = errors.New("cart not found")
	ErrOverflow        = errors.New("cart value exceeds uint32")
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

type Cart struct {
	Items      []Item `json:"items"`
	TotalPrice uint32 `json:"total_price"`
}

type cartData struct {
	items map[int64]Item
	total uint32
}

type Service struct {
	client  ProductClient
	mu      sync.RWMutex
	carts   map[int64]*cartData
	timeout time.Duration
}

func NewService(client ProductClient) *Service {
	return &Service{client: client, carts: make(map[int64]*cartData), timeout: 3 * time.Second}
}

func (s *Service) lookup(ctx context.Context, sku int64) (*productpb.Product, error) {
	p, err := s.client.GetProduct(ctx, &productpb.GetProductRequest{Sku: sku})
	if err != nil {
		return nil, err
	}
	if p == nil || p.GetSku() != sku || p.GetPrice() < 0 {
		return nil, errors.New("invalid product service response")
	}
	return p, nil
}

func (s *Service) AddToCart(ctx context.Context, userID, sku int64, count uint32) error {
	if userID <= 0 || sku <= 0 || count == 0 {
		return ErrInvalidArgument
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	p, err := s.lookup(ctx, sku)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.carts[userID]
	if c == nil {
		c = &cartData{items: make(map[int64]Item)}
	}
	previous := c.items[sku]
	newCount := uint64(previous.Count) + uint64(count)
	newTotal := uint64(c.total) - uint64(previous.Price)*uint64(previous.Count) + uint64(p.GetPrice())*newCount
	if newCount > math.MaxUint32 || newTotal > math.MaxUint32 {
		return ErrOverflow
	}
	c.items[sku] = Item{SKU: sku, Name: p.GetName(), Count: uint32(newCount), Price: uint32(p.GetPrice())}
	c.total = uint32(newTotal)
	s.carts[userID] = c
	return nil
}

func (s *Service) snapshot(userID int64) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.carts[userID]
	if c == nil {
		return nil
	}
	items := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SKU < items[j].SKU })
	return items
}

func (s *Service) GetCart(ctx context.Context, userID int64) (Cart, error) {
	if userID <= 0 {
		return Cart{}, ErrInvalidArgument
	}
	snapshot := s.snapshot(userID)
	if len(snapshot) == 0 {
		return Cart{}, ErrCartNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	items := make([]Item, len(snapshot))
	errs := make([]error, len(snapshot))
	var wg sync.WaitGroup
	wg.Add(len(snapshot))
	for i, line := range snapshot {
		go func() {
			defer wg.Done()
			p, err := s.lookup(ctx, line.SKU)
			if err != nil {
				errs[i] = err
				return
			}
			items[i] = Item{SKU: line.SKU, Name: p.GetName(), Count: line.Count, Price: uint32(p.GetPrice())}
		}()
	}
	wg.Wait()
	var total uint64
	for i, err := range errs {
		if err != nil {
			return Cart{}, err
		}
		total += uint64(items[i].Price) * uint64(items[i].Count)
		if total > math.MaxUint32 {
			return Cart{}, ErrOverflow
		}
	}
	return Cart{Items: items, TotalPrice: uint32(total)}, nil
}
