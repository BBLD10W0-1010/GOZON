package domain

import (
	"errors"
	"fmt"
	"time"
)

type ID uint64
type Status string

const (
	Planned    Status = "planned"
	InProgress Status = "in_progress"
	Canceled   Status = "canceled"
	Done       Status = "done"
)

var (
	ErrNotFound   = errors.New("задача не найдена")
	ErrTerminal   = errors.New("завершённую или отменённую задачу нельзя изменять")
	ErrSameStatus = errors.New("задача уже имеет этот статус")
	ErrInvalid    = errors.New("некорректные данные")
)

type Task struct {
	ID       ID        `json:"id"`
	Title    string    `json:"title"`
	Status   Status    `json:"status"`
	Deadline time.Time `json:"deadline"`
}

type State struct {
	NextID ID     `json:"next_id"`
	Tasks  []Task `json:"tasks"`
}

func (s Status) Valid() bool {
	return s == Planned || s == InProgress || s == Canceled || s == Done
}

func (s Status) Terminal() bool { return s == Done || s == Canceled }

func ParseDeadline(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("дедлайн должен иметь формат YYYY-MM-DD: %w", err)
	}
	return date, nil
}

func Today(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
