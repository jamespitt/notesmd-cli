package mocks

import (
	"os"
	"time"
)

// MockFileInfo implements os.FileInfo for testing.
type MockFileInfo struct {
	FileName    string
	IsDirectory bool
}

func (m *MockFileInfo) Name() string      { return m.FileName }
func (m *MockFileInfo) Size() int64       { return 0 }
func (m *MockFileInfo) Mode() os.FileMode { return 0644 }
func (m *MockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *MockFileInfo) IsDir() bool       { return m.IsDirectory }
func (m *MockFileInfo) Sys() any          { return nil }
