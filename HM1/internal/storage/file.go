package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"hm1/internal/domain"
	"os"
	"path/filepath"
	"strings"
)

type File struct{ path string }

func NewFile(path string) (*File, error) {
	f := &File{path: path}
	if _, err := f.Load(); err != nil {
		return nil, fmt.Errorf("открыть хранилище: %w", err)
	}
	return f, nil
}

func (f *File) Load() (domain.State, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.State{NextID: 1, Tasks: []domain.Task{}}, nil
	}
	if err != nil {
		return domain.State{}, fmt.Errorf("прочитать файл: %w", err)
	}
	var state domain.State
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.State{}, fmt.Errorf("прочитать JSON хранилища: %w", err)
	}
	if err := validate(state); err != nil {
		return domain.State{}, fmt.Errorf("проверить хранилище: %w", err)
	}
	return state, nil
}

func validate(s domain.State) error {
	if s.NextID == 0 {
		return fmt.Errorf("next_id должен быть положительным: %w", domain.ErrInvalid)
	}
	seen := make(map[domain.ID]bool)
	for _, t := range s.Tasks {
		if t.ID == 0 || t.ID >= s.NextID || seen[t.ID] || strings.TrimSpace(t.Title) == "" || !t.Status.Valid() || t.Deadline.IsZero() {
			return fmt.Errorf("задача %d в хранилище: %w", t.ID, domain.ErrInvalid)
		}
		if t.Deadline.Location() != nil && (t.Deadline.Hour() != 0 || t.Deadline.Minute() != 0 || t.Deadline.Second() != 0 || t.Deadline.Nanosecond() != 0 || t.Deadline.Format("Z07:00") != "Z") {
			return fmt.Errorf("дедлайн задачи %d должен быть датой в UTC: %w", t.ID, domain.ErrInvalid)
		}
		seen[t.ID] = true
	}
	return nil
}

func (f *File) Save(s domain.State) error {
	if err := validate(s); err != nil {
		return fmt.Errorf("проверить данные: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("сериализовать хранилище: %w", err)
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("создать каталог: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tasks-*.tmp")
	if err != nil {
		return fmt.Errorf("создать временный файл: %w", err)
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(append(data, '\n'))
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if writeErr != nil {
		return fmt.Errorf("записать хранилище: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрыть хранилище: %w", closeErr)
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return fmt.Errorf("заменить хранилище: %w", err)
	}
	return nil
}
