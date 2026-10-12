package storage_test

import (
	"hm1/internal/domain"
	"hm1/internal/service"
	"hm1/internal/storage"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistenceAndIDsAfterRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tasks.json")
	repo, err := storage.NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	s := service.New(repo, clock)
	first, err := s.Add("Persistent", clock())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s = service.New(reopened, clock)
	got, err := s.Get(first.ID)
	if err != nil || got != first {
		t.Fatalf("restart: %+v %v", got, err)
	}
	if err := s.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err = storage.NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.New(reopened, clock).Add("Next", clock())
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID+1 {
		t.Fatalf("reused ID: %d", second.ID)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Save(domain.State{}); err == nil {
		t.Fatal("invalid state accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("invalid save changed file")
	}
}

func TestCorruptedStorageRejected(t *testing.T) {
	for _, data := range []string{`{`, `null`, `{"next_id":0}`, `{"next_id":2,"tasks":[{"id":2,"title":"Task","status":"planned","deadline":"2030-01-01T00:00:00Z"}]}`, `{"next_id":3,"tasks":[{"id":1,"title":"Task","status":"bad","deadline":"2030-01-01T00:00:00Z"}]}`} {
		path := filepath.Join(t.TempDir(), "tasks.json")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := storage.NewFile(path); err == nil {
			t.Fatalf("accepted corrupt file: %s", data)
		}
	}
}

func TestMemoryDoesNotLeakMutableSlices(t *testing.T) {
	repo, err := storage.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	state := domain.State{NextID: 2, Tasks: []domain.Task{{ID: 1, Title: "Original", Status: domain.Planned, Deadline: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}}}
	if err := repo.Save(state); err != nil {
		t.Fatal(err)
	}
	state.Tasks[0].Title = "Changed"
	loaded, err := repo.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].Title != "Original" {
		t.Fatal("Save retained caller slice")
	}
	loaded.Tasks[0].Title = "Changed again"
	loaded, err = repo.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tasks[0].Title != "Original" {
		t.Fatal("Load exposed internal slice")
	}
}
