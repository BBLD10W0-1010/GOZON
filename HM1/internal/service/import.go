package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hm1/internal/domain"
	"math"
	"strings"
)

type ImportIssue struct {
	Index int
	Err   error
}
type ImportResult struct {
	Added  int
	Issues []ImportIssue
}

// ImportJSON uses a separate input format: IDs are always assigned by the tracker.
func (s *Service) ImportJSON(data []byte) (ImportResult, error) {
	var rows []json.RawMessage
	if strings.TrimSpace(string(data)) == "null" {
		return ImportResult{}, fmt.Errorf("ожидается JSON-массив: %w", domain.ErrInvalid)
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return ImportResult{}, fmt.Errorf("прочитать JSON импорта: %w", err)
	}
	state, err := s.load()
	if err != nil {
		return ImportResult{}, err
	}
	result := ImportResult{}
	for i, row := range rows {
		var input struct {
			Title    string        `json:"title"`
			Deadline string        `json:"deadline"`
			Status   domain.Status `json:"status"`
		}
		decoder := json.NewDecoder(bytes.NewReader(row))
		decoder.DisallowUnknownFields()
		rowErr := decoder.Decode(&input)
		deadline, dateErr := domain.ParseDeadline(input.Deadline)
		if rowErr == nil {
			rowErr = dateErr
		}
		if rowErr == nil {
			rowErr = s.validate(input.Title, deadline)
		}
		if input.Status == "" {
			input.Status = domain.Planned
		}
		if rowErr == nil && !input.Status.Valid() {
			rowErr = fmt.Errorf("неизвестный статус %q: %w", input.Status, domain.ErrInvalid)
		}
		if rowErr == nil && (state.NextID == 0 || uint64(state.NextID) == math.MaxUint64) {
			rowErr = fmt.Errorf("идентификаторы исчерпаны: %w", domain.ErrInvalid)
		}
		if rowErr != nil {
			result.Issues = append(result.Issues, ImportIssue{Index: i + 1, Err: fmt.Errorf("запись %d: %w", i+1, rowErr)})
			continue
		}
		state.Tasks = append(state.Tasks, domain.Task{ID: state.NextID, Title: strings.TrimSpace(input.Title), Deadline: deadline, Status: input.Status})
		state.NextID++
		result.Added++
	}
	if result.Added > 0 {
		if err := s.save(state); err != nil {
			return ImportResult{}, fmt.Errorf("сохранить импорт: %w", err)
		}
	}
	return result, nil
}
