package obsidian

import (
	"fmt"
	"net/url"

	open "github.com/skratchdot/open-golang/open"
)

// Uri implements UriManager using the system's default URI handler.
type Uri struct{}

// Construct builds an obsidian:// URI with the given base URL and parameters.
func (u *Uri) Construct(baseUrl string, params map[string]string) string {
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}
	return fmt.Sprintf("%s?%s", baseUrl, query.Encode())
}

// Execute opens the given URI with the system's default handler.
func (u *Uri) Execute(uri string) error {
	return open.Run(uri)
}
