package storage

import "hm1/internal/domain"

type Memory struct{ state domain.State }

func NewMemory() (*Memory, error) {
	return &Memory{state: domain.State{NextID: 1, Tasks: []domain.Task{}}}, nil
}

func clone(s domain.State) domain.State {
	s.Tasks = append([]domain.Task{}, s.Tasks...)
	return s
}

func (m *Memory) Load() (domain.State, error) { return clone(m.state), nil }
func (m *Memory) Save(s domain.State) error {
	if err := validate(s); err != nil {
		return err
	}
	m.state = clone(s)
	return nil
}
