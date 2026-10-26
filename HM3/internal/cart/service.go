package cart

import (
	"context"
	"errors"
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
	return s.addItem(userID, productItem(p, count))
}

func (s *Service) addItem(userID int64, item Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.carts[userID]
	if c == nil {
		c = &cartData{items: make(map[int64]Item)}
	}
	if err := c.add(item); err != nil {
		return err
	}
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
	sort.Slice(snapshot, func(i, j int) bool { return snapshot[i].SKU < snapshot[j].SKU })
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	items, err := s.fetchItems(ctx, snapshot)
	if err != nil {
		return Cart{}, err
	}
	return makeCart(items)
}

func (s *Service) fetchItems(ctx context.Context, snapshot []Item) ([]Item, error) {
	items := make([]Item, len(snapshot))
	errs := make([]error, len(snapshot))
	var wg sync.WaitGroup
	wg.Add(len(snapshot))
	for i, line := range snapshot {
		go func() {
			defer wg.Done()
			items[i], errs[i] = s.refreshItem(ctx, line)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) refreshItem(ctx context.Context, line Item) (Item, error) {
	p, err := s.lookup(ctx, line.SKU)
	if err != nil {
		return Item{}, err
	}
	return productItem(p, line.Count), nil
}
