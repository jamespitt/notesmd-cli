package mocks

import "fmt"

type MockUriManager struct {
	ExecuteErr  error
	LastParams  map[string]string
	LastBaseUrl string
}

func (m *MockUriManager) Construct(baseUrl string, params map[string]string) string {
	m.LastBaseUrl = baseUrl
	m.LastParams = params
	query := ""
	for k, v := range params {
		if query != "" {
			query += "&"
		}
		query += fmt.Sprintf("%s=%s", k, v)
	}
	return baseUrl + "?" + query
}

func (m *MockUriManager) Execute(_ string) error {
	return m.ExecuteErr
}
