package hub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiURL   string
	apiToken string
	http     *http.Client
}

type User struct {
	Name    string            `json:"name"`
	Servers map[string]Server `json:"servers"`
}

type Server struct {
	Name        string            `json:"name"`
	Ready       bool              `json:"ready"`
	Pending     any               `json:"pending"`
	URL         string            `json:"url"`
	UserOptions map[string]string `json:"user_options"`
}

func NewClient(apiURL, apiToken string) *Client {
	return &Client{
		apiURL:   apiURL,
		apiToken: apiToken,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.apiURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func (c *Client) ListUsers() ([]User, error) {
	resp, err := c.do(http.MethodGet, "/users", nil)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list users failed (%d): %s", resp.StatusCode, body)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decoding users: %w", err)
	}
	return users, nil
}

func (c *Client) GetUser(username string) (*User, error) {
	resp, err := c.do(http.MethodGet, "/users/"+username, nil)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user failed (%d): %s", resp.StatusCode, body)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding user: %w", err)
	}
	return &user, nil
}

func (c *Client) EnsureUser(username string) error {
	resp, err := c.do(http.MethodPost, "/users/"+username, nil)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	defer resp.Body.Close()
	// 201 = created, 409 = already exists — both are fine
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create user failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

func (c *Client) SpawnServer(username, serverName, profile string) error {
	body := map[string]string{"profile": profile}
	resp, err := c.do(http.MethodPost, "/users/"+username+"/servers/"+serverName, body)
	if err != nil {
		return fmt.Errorf("spawning server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("spawn failed (%d): %s", resp.StatusCode, b)
	}
	return nil
}

func (c *Client) CreateUserToken(username string) (string, error) {
	body := map[string]any{"expires_in": 300, "note": "tostada-connect"}
	resp, err := c.do(http.MethodPost, "/users/"+username+"/tokens", body)
	if err != nil {
		return "", fmt.Errorf("creating token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create token failed (%d): %s", resp.StatusCode, b)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding token: %w", err)
	}
	return result.Token, nil
}

func (c *Client) ServerProgress(username, serverName string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.apiURL+"/users/"+username+"/servers/"+serverName+"/progress", nil)
	if err != nil {
		return nil, fmt.Errorf("creating progress request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.apiToken)
	// Use a client without the default 10s timeout — SSE streams are long-lived
	return (&http.Client{}).Do(req)
}

func (c *Client) StopServer(username, serverName string) error {
	resp, err := c.do(http.MethodDelete, "/users/"+username+"/servers/"+serverName, nil)
	if err != nil {
		return fmt.Errorf("stopping server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop failed (%d): %s", resp.StatusCode, b)
	}
	return nil
}
