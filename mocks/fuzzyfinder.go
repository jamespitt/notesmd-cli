package mocks

type MockFuzzyFinder struct {
	FindErr       error
	SelectedIndex int
}

func (m *MockFuzzyFinder) Find(items []string, _ func(i int) string) (int, error) {
	if m.FindErr != nil {
		return 0, m.FindErr
	}
	return m.SelectedIndex, nil
}
