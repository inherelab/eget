package configutil

import (
	"io"

	gconfig "github.com/gookit/config/v2"
)

type Manager struct {
	inner *gconfig.Config
}

func NewManager(name string) *Manager {
	return &Manager{inner: NewTOMLManager(name)}
}

func LoadManager(name, path string) (*Manager, error) {
	cfg, err := LoadTOMLFile(name, path)
	if err != nil {
		return nil, err
	}
	return &Manager{inner: cfg}, nil
}

func (m *Manager) Config() *gconfig.Config {
	if m == nil {
		return nil
	}
	return m.inner
}

func (m *Manager) Set(key string, value any, byPath bool) error {
	return m.inner.Set(key, value, byPath)
}

func (m *Manager) SetData(data map[string]any) {
	m.inner.SetData(data)
}

func (m *Manager) GetValue(key string, byPath bool) (any, bool) {
	return m.inner.GetValue(key, byPath)
}

func (m *Manager) Exists(key string, byPath bool) bool {
	return m.inner.Exists(key, byPath)
}

func (m *Manager) BindStruct(key string, dst any) error {
	return m.inner.BindStruct(key, dst)
}

func (m *Manager) MapOnExists(key string, dst any) error {
	return m.inner.MapOnExists(key, dst)
}

func (m *Manager) Decode(dst any) error {
	return m.inner.Decode(dst)
}

func (m *Manager) Data() map[string]any {
	return m.inner.Data()
}

func (m *Manager) DumpTo(out io.Writer) (int64, error) {
	return m.inner.DumpTo(out, FormatTOML)
}

func (m *Manager) SaveTo(path string) error {
	return SaveTOMLFile(path, m.inner)
}
