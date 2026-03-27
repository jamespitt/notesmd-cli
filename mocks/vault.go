package mocks

type MockVaultOperator struct {
	DefaultNameErr         error
	PathError              error
	Name                   string
	PathValue              string
	OpenType               string
	OpenTypeErr            error
	DefaultTaskFolders     []string
	TaskFoldersErr         error
	DefaultProjectsFolder  string
	ProjectsFolderErr      error
	DefaultCalendarFolder  string
	CalendarFolderErr      error
}

func (m *MockVaultOperator) DefaultName() (string, error) {
	if m.DefaultNameErr != nil {
		return "", m.DefaultNameErr
	}
	return m.Name, nil
}

func (m *MockVaultOperator) SetDefaultName(_ string) error {
	if m.DefaultNameErr != nil {
		return m.DefaultNameErr
	}
	return nil
}

func (m *MockVaultOperator) Path() (string, error) {
	if m.PathError != nil {
		return "", m.PathError
	}
	if m.PathValue != "" {
		return m.PathValue, nil
	}
	return "path", nil
}

func (m *MockVaultOperator) DefaultOpenType() (string, error) {
	if m.OpenTypeErr != nil {
		return "", m.OpenTypeErr
	}
	if m.OpenType != "" {
		return m.OpenType, nil
	}
	return "obsidian", nil
}

func (m *MockVaultOperator) TaskFolders() ([]string, error) {
	if m.TaskFoldersErr != nil {
		return nil, m.TaskFoldersErr
	}
	return m.DefaultTaskFolders, nil
}

func (m *MockVaultOperator) ProjectsFolder() (string, error) {
	if m.ProjectsFolderErr != nil {
		return "", m.ProjectsFolderErr
	}
	if m.DefaultProjectsFolder != "" {
		return m.DefaultProjectsFolder, nil
	}
	return "Projects", nil
}

func (m *MockVaultOperator) CalendarFolder() (string, error) {
	if m.CalendarFolderErr != nil {
		return "", m.CalendarFolderErr
	}
	if m.DefaultCalendarFolder != "" {
		return m.DefaultCalendarFolder, nil
	}
	return "Journal/Calendar", nil
}
