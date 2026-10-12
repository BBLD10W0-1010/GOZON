package service

import (
	"fmt"
	"hm1/internal/domain"
	"math"
	"sort"
	"strings"
	"time"
)

type Repository interface {
	Load() (domain.State, error)
	Save(domain.State) error
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func New(repo Repository, now func() time.Time) *Service { return &Service{repo: repo, now: now} }

func (s *Service) load() (domain.State, error) {
	state, err := s.repo.Load()
	if err != nil {
		return domain.State{}, fmt.Errorf("загрузить задачи: %w", err)
	}
	return state, nil
}

func (s *Service) save(state domain.State) error {
	if err := s.repo.Save(state); err != nil {
		return fmt.Errorf("сохранить задачи: %w", err)
	}
	return nil
}

func (s *Service) validate(title string, deadline time.Time) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("заголовок не может быть пустым: %w", domain.ErrInvalid)
	}
	if deadline.IsZero() || deadline.Before(domain.Today(s.now())) {
		return fmt.Errorf("дедлайн не может быть в прошлом: %w", domain.ErrInvalid)
	}
	if !deadline.Equal(domain.Today(deadline)) {
		return fmt.Errorf("дедлайн должен содержать только дату в UTC: %w", domain.ErrInvalid)
	}
	return nil
}

func (s *Service) Add(title string, deadline time.Time) (domain.Task, error) {
	if err := s.validate(title, deadline); err != nil {
		return domain.Task{}, fmt.Errorf("добавить задачу: %w", err)
	}
	state, err := s.load()
	if err != nil {
		return domain.Task{}, err
	}
	if state.NextID == 0 || uint64(state.NextID) == math.MaxUint64 {
		return domain.Task{}, fmt.Errorf("идентификаторы исчерпаны: %w", domain.ErrInvalid)
	}
	t := domain.Task{ID: state.NextID, Title: strings.TrimSpace(title), Status: domain.Planned, Deadline: deadline.UTC()}
	state.NextID++
	state.Tasks = append(state.Tasks, t)
	if err := s.save(state); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

func find(state domain.State, id domain.ID) (int, error) {
	for i, t := range state.Tasks {
		if t.ID == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("ID %d: %w", id, domain.ErrNotFound)
}

func (s *Service) Get(id domain.ID) (domain.Task, error) {
	state, err := s.load()
	if err != nil {
		return domain.Task{}, err
	}
	i, err := find(state, id)
	if err != nil {
		return domain.Task{}, fmt.Errorf("получить задачу: %w", err)
	}
	return state.Tasks[i], nil
}

func (s *Service) ChangeStatus(id domain.ID, status domain.Status) error {
	if !status.Valid() {
		return fmt.Errorf("неизвестный статус %q: %w", status, domain.ErrInvalid)
	}
	state, err := s.load()
	if err != nil {
		return err
	}
	i, err := find(state, id)
	if err != nil {
		return fmt.Errorf("изменить статус: %w", err)
	}
	if state.Tasks[i].Status.Terminal() {
		return fmt.Errorf("ID %d: %w", id, domain.ErrTerminal)
	}
	if state.Tasks[i].Status == status {
		return fmt.Errorf("ID %d: %w", id, domain.ErrSameStatus)
	}
	state.Tasks[i].Status = status
	return s.save(state)
}

type Edit struct {
	Title    *string
	Deadline *time.Time
}

func (s *Service) Edit(id domain.ID, change Edit) error {
	if change.Title == nil && change.Deadline == nil {
		return fmt.Errorf("укажите заголовок и/или дедлайн: %w", domain.ErrInvalid)
	}
	state, err := s.load()
	if err != nil {
		return err
	}
	i, err := find(state, id)
	if err != nil {
		return fmt.Errorf("редактировать задачу: %w", err)
	}
	t := state.Tasks[i]
	if t.Status.Terminal() {
		return fmt.Errorf("ID %d: %w", id, domain.ErrTerminal)
	}
	if change.Title != nil {
		if strings.TrimSpace(*change.Title) == "" {
			return fmt.Errorf("заголовок не может быть пустым: %w", domain.ErrInvalid)
		}
		t.Title = strings.TrimSpace(*change.Title)
	}
	if change.Deadline != nil {
		if err := s.validate(t.Title, *change.Deadline); err != nil {
			return fmt.Errorf("редактировать задачу: %w", err)
		}
		t.Deadline = change.Deadline.UTC()
	}
	state.Tasks[i] = t
	return s.save(state)
}

func (s *Service) Delete(id domain.ID) error {
	state, err := s.load()
	if err != nil {
		return err
	}
	i, err := find(state, id)
	if err != nil {
		return fmt.Errorf("удалить задачу: %w", err)
	}
	state.Tasks = append(state.Tasks[:i], state.Tasks[i+1:]...)
	return s.save(state)
}

type Filter struct {
	Status      domain.Status
	Overdue     bool
	Search      string
	Page, Limit int
}
type Result struct {
	Tasks              []domain.Task
	Total, Pages, Page int
}

func (s *Service) List(f Filter) (Result, error) {
	if f.Page == 0 {
		f.Page = 1
	}
	if f.Limit == 0 {
		f.Limit = 20
	}
	if f.Page < 1 || f.Limit < 1 || (f.Status != "" && !f.Status.Valid()) {
		return Result{}, fmt.Errorf("неверные параметры списка: %w", domain.ErrInvalid)
	}
	state, err := s.load()
	if err != nil {
		return Result{}, err
	}
	items := make([]domain.Task, 0)
	query := strings.ToLower(f.Search)
	today := domain.Today(s.now())
	for _, t := range state.Tasks {
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.Overdue && (!t.Deadline.Before(today) || t.Status.Terminal()) {
			continue
		}
		if !strings.Contains(strings.ToLower(t.Title), query) {
			continue
		}
		items = append(items, t)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Deadline.Equal(items[j].Deadline) {
			return items[i].ID < items[j].ID
		}
		return items[i].Deadline.Before(items[j].Deadline)
	})
	r := Result{Tasks: []domain.Task{}, Total: len(items), Page: f.Page}
	if r.Total == 0 {
		return r, nil
	}
	r.Pages = (r.Total-1)/f.Limit + 1
	if f.Page > r.Pages {
		return r, nil
	}
	start := (f.Page - 1) * f.Limit
	end := r.Total
	if f.Limit < r.Total-start {
		end = start + f.Limit
	}
	r.Tasks = items[start:end]
	return r, nil
}
