package credentials

import "context"

// Memory is an in-process test double.
type Memory struct {
	Secrets map[string]string
	Fail    error
	Calls   int
}

func (m *Memory) Kind() string { return "memory" }

func (m *Memory) Exists(_ context.Context, account string) (bool, error) {
	m.Calls++
	if m.Fail != nil {
		return false, m.Fail
	}
	if m.Secrets == nil {
		return false, nil
	}
	_, ok := m.Secrets[account]
	return ok, nil
}

func (m *Memory) Get(_ context.Context, account string) (string, error) {
	m.Calls++
	if m.Fail != nil {
		return "", m.Fail
	}
	if m.Secrets == nil {
		return "", ErrNotFound
	}
	secret, ok := m.Secrets[account]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (m *Memory) Set(_ context.Context, account, secret string) error {
	m.Calls++
	if m.Fail != nil {
		return m.Fail
	}
	if m.Secrets == nil {
		m.Secrets = map[string]string{}
	}
	m.Secrets[account] = secret
	return nil
}

func (m *Memory) Delete(_ context.Context, account string) error {
	m.Calls++
	if m.Fail != nil {
		return m.Fail
	}
	if m.Secrets == nil {
		return ErrNotFound
	}
	if _, ok := m.Secrets[account]; !ok {
		return ErrNotFound
	}
	delete(m.Secrets, account)
	return nil
}
