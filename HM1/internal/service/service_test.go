package service_test

import (
	"errors"
	"hm1/internal/domain"
	"hm1/internal/service"
	"hm1/internal/storage"
	"reflect"
	"testing"
	"time"
)

func setup(t *testing.T) (*service.Service, *time.Time) {
	t.Helper()
	repo, err := storage.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 10, 12, 0, 0, 0, time.FixedZone("UTC+5", 5*3600))
	return service.New(repo, func() time.Time { return now }), &now
}

func date(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := domain.ParseDeadline(value)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func add(t *testing.T, s *service.Service, title, deadline string) domain.Task {
	t.Helper()
	task, err := s.Add(title, date(t, deadline))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestCreateAndNeverReuseIDs(t *testing.T) {
	s, _ := setup(t)
	for _, tc := range []struct{ title, deadline string }{{"", "2030-01-10"}, {"  ", "2030-01-10"}, {"Task", "2030-01-09"}} {
		if _, err := s.Add(tc.title, date(t, tc.deadline)); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("expected invalid, got %v", err)
		}
	}
	first := add(t, s, " First ", "2030-01-10")
	if first.ID != 1 || first.Title != "First" || first.Status != domain.Planned {
		t.Fatalf("unexpected task: %+v", first)
	}
	if err := s.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	second := add(t, s, "Second", "2030-01-11")
	if second.ID != 2 {
		t.Fatalf("reused ID: %d", second.ID)
	}
	if _, err := s.Get(first.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := s.Delete(first.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestStatusAndEditRules(t *testing.T) {
	for _, terminal := range []domain.Status{domain.Done, domain.Canceled} {
		t.Run(string(terminal), func(t *testing.T) {
			s, _ := setup(t)
			task := add(t, s, "Task", "2030-01-10")
			if err := s.ChangeStatus(task.ID, domain.Planned); !errors.Is(err, domain.ErrSameStatus) {
				t.Fatal(err)
			}
			if err := s.ChangeStatus(task.ID, "unknown"); !errors.Is(err, domain.ErrInvalid) {
				t.Fatal(err)
			}
			if err := s.ChangeStatus(task.ID, domain.InProgress); err != nil {
				t.Fatal(err)
			}
			blank := " "
			past := date(t, "2030-01-09")
			for _, edit := range []service.Edit{{}, {Title: &blank}, {Deadline: &past}} {
				if err := s.Edit(task.ID, edit); !errors.Is(err, domain.ErrInvalid) {
					t.Fatal(err)
				}
			}
			title := "Updated"
			deadline := date(t, "2030-01-12")
			if err := s.Edit(task.ID, service.Edit{Title: &title, Deadline: &deadline}); err != nil {
				t.Fatal(err)
			}
			got, err := s.Get(task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != title || !got.Deadline.Equal(deadline) {
				t.Fatalf("edit not applied: %+v", got)
			}
			if err := s.ChangeStatus(task.ID, terminal); err != nil {
				t.Fatal(err)
			}
			if err := s.ChangeStatus(task.ID, domain.Planned); !errors.Is(err, domain.ErrTerminal) {
				t.Fatal(err)
			}
			if err := s.Edit(task.ID, service.Edit{Title: &title}); !errors.Is(err, domain.ErrTerminal) {
				t.Fatal(err)
			}
			if err := s.Delete(task.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestListFiltersSortingAndPagination(t *testing.T) {
	s, now := setup(t)
	add(t, s, "Позже", "2030-01-12")
	add(t, s, "Изучить GO", "2030-01-10")
	add(t, s, "Изучить Go дальше", "2030-01-10")
	finished := add(t, s, "Завершить", "2030-01-10")
	if err := s.ChangeStatus(finished.ID, domain.Done); err != nil {
		t.Fatal(err)
	}
	*now = now.AddDate(0, 0, 1)
	r, err := s.List(service.Filter{Overdue: true, Search: "изучить go", Limit: 1, Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 2 || r.Pages != 2 || len(r.Tasks) != 1 || r.Tasks[0].ID != 3 {
		t.Fatalf("unexpected page: %+v", r)
	}
	r, err = s.List(service.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []domain.ID
	for _, task := range r.Tasks {
		ids = append(ids, task.ID)
	}
	if !reflect.DeepEqual(ids, []domain.ID{2, 3, 4, 1}) {
		t.Fatalf("bad order: %v", ids)
	}
	r, err = s.List(service.Filter{Status: domain.Done})
	if err != nil || r.Total != 1 {
		t.Fatalf("status filter: %+v %v", r, err)
	}
	r, err = s.List(service.Filter{Page: int(^uint(0) >> 1), Limit: int(^uint(0) >> 1)})
	if err != nil || len(r.Tasks) != 0 {
		t.Fatalf("large page: %+v %v", r, err)
	}
	for _, filter := range []service.Filter{{Page: -1}, {Limit: -1}, {Status: "unknown"}} {
		if _, err := s.List(filter); !errors.Is(err, domain.ErrInvalid) {
			t.Fatal(err)
		}
	}
	newTitle := "Просроченная задача"
	if err := s.Edit(2, service.Edit{Title: &newTitle}); err != nil {
		t.Fatalf("title-only overdue edit: %v", err)
	}
}

func TestImportContinuesAfterInvalidRows(t *testing.T) {
	s, _ := setup(t)
	r, err := s.ImportJSON([]byte(`[
		{"title":"One","deadline":"2030-01-10"},
		{"title":"","deadline":"2030-01-11"},
		{"title":"Old","deadline":"2029-01-01"},
		{"title":"Bad status","deadline":"2030-01-11","status":"bad"},
		{"title":"Two","deadline":"2030-01-11","status":"in_progress"},
		{"title":"Extra","deadline":"2030-01-11","id":1},
		42
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Added != 2 || len(r.Issues) != 5 || r.Issues[0].Index != 2 || r.Issues[4].Index != 7 {
		t.Fatalf("unexpected import: %+v", r)
	}
	got, err := s.Get(2)
	if err != nil || got.Title != "Two" || got.Status != domain.InProgress {
		t.Fatalf("valid row lost: %+v %v", got, err)
	}
	for _, data := range []string{`{`, `null`, `{}`} {
		if _, err := s.ImportJSON([]byte(data)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

type brokenRepo struct {
	cause    error
	failSave bool
}

func (r brokenRepo) Load() (domain.State, error) {
	if !r.failSave {
		return domain.State{}, r.cause
	}
	return domain.State{NextID: 1}, nil
}
func (r brokenRepo) Save(domain.State) error { return r.cause }

func TestStorageErrorsAreWrapped(t *testing.T) {
	cause := errors.New("disk failure")
	for _, failSave := range []bool{false, true} {
		s := service.New(brokenRepo{cause: cause, failSave: failSave}, func() time.Time { return date(t, "2030-01-10") })
		if _, err := s.Add("Task", date(t, "2030-01-10")); !errors.Is(err, cause) {
			t.Fatalf("error lost: %v", err)
		}
	}
}
