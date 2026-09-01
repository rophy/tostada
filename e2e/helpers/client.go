//go:build e2e

package helpers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
)

const defaultBaseURL = "http://localhost:12025"

func BaseURL() string {
	if v := os.Getenv("TOSTADA_E2E_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultBaseURL
}

type Client struct {
	HTTP    *http.Client
	BaseURL string
	T       *testing.T
}

func NewClient(t *testing.T) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &Client{
		HTTP: &http.Client{
			Jar: jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		BaseURL: BaseURL(),
		T:       t,
	}
}

// toLocal rewrites an absolute URL (which may point to the external domain)
// to go through the local base URL instead.
func (c *Client) toLocal(rawURL string) string {
	if strings.HasPrefix(rawURL, "/") {
		return c.BaseURL + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	base, _ := url.Parse(c.BaseURL)
	parsed.Scheme = base.Scheme
	parsed.Host = base.Host
	return parsed.String()
}

// Login performs the full OIDC login flow against the mock provider.
func (c *Client) Login(username string) {
	c.T.Helper()

	// Step 1: GET /api/auth/login — redirects to /authorize?... on the external domain
	resp, err := c.HTTP.Get(c.BaseURL + "/api/auth/login")
	if err != nil {
		c.T.Fatalf("login request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		c.T.Fatalf("login: expected 302, got %d", resp.StatusCode)
	}

	authorizeURL := c.toLocal(resp.Header.Get("Location"))
	if authorizeURL == "" {
		c.T.Fatal("login: no Location header")
	}

	// Step 2: GET the authorize page (user selection form)
	resp, err = c.HTTP.Get(authorizeURL)
	if err != nil {
		c.T.Fatalf("authorize GET: %v", err)
	}
	resp.Body.Close()

	// Step 3: POST to /authorize/callback with the selected user's sub.
	// The oidc-mock form fields: sub, client_id, redirect_uri, state, nonce.
	parsedAuth, _ := url.Parse(authorizeURL)
	q := parsedAuth.Query()

	// Find the sub value for the requested username.
	// The mock maps sub=alice→preferred_username=alice, sub=bob→preferred_username=bob.
	sub := username

	formData := url.Values{
		"sub":          {sub},
		"client_id":    {q.Get("client_id")},
		"redirect_uri": {q.Get("redirect_uri")},
		"state":        {q.Get("state")},
		"nonce":        {q.Get("nonce")},
	}

	resp, err = c.HTTP.PostForm(c.BaseURL+"/authorize/callback", formData)
	if err != nil {
		c.T.Fatalf("authorize POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		c.T.Fatalf("authorize POST: expected 302, got %d", resp.StatusCode)
	}

	// Step 4: Follow redirect to /api/auth/callback?code=...&state=...
	// The Location may point to the external domain — rewrite to localhost.
	callbackURL := c.toLocal(resp.Header.Get("Location"))
	if callbackURL == "" {
		c.T.Fatal("authorize: no callback Location")
	}

	resp, err = c.HTTP.Get(callbackURL)
	if err != nil {
		c.T.Fatalf("callback GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		body := "(no body)"
		c.T.Fatalf("callback: expected 302, got %d; body: %s", resp.StatusCode, body)
	}

	// Verify session cookie
	u, _ := url.Parse(c.BaseURL)
	for _, cookie := range c.HTTP.Jar.Cookies(u) {
		if cookie.Name == "tostada_session" {
			return
		}
	}
	c.T.Fatal("login: no tostada_session cookie after callback")
}

func (c *Client) Get(path string) *http.Response {
	c.T.Helper()
	resp, err := c.HTTP.Get(c.BaseURL + path)
	if err != nil {
		c.T.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (c *Client) PostJSON(path, body string) *http.Response {
	c.T.Helper()
	resp, err := c.HTTP.Post(c.BaseURL+path, "application/json", strings.NewReader(body))
	if err != nil {
		c.T.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (c *Client) Delete(path string) *http.Response {
	c.T.Helper()
	req, err := http.NewRequest("DELETE", c.BaseURL+path, nil)
	if err != nil {
		c.T.Fatalf("DELETE %s: %v", path, err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		c.T.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

func (c *Client) ExpectStatus(resp *http.Response, want int) {
	c.T.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		c.T.Fatalf("%s %s: status %d, want %d; body: %s",
			resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, string(body))
	}
}

func NewUnauthenticatedClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func RequireCluster(t *testing.T) {
	t.Helper()
	base := BaseURL()
	resp, err := http.Get(base + "/api/auth/login")
	if err != nil {
		t.Skipf("cluster not available at %s: %v", base, err)
	}
	resp.Body.Close()
	fmt.Fprintf(os.Stderr, "e2e: using %s\n", base)
}
