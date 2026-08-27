package hub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Pending bool   `json:"pending"`
	URL     string `json:"url"`
}

func NewClient(apiURL, apiToken string) *Client {
	return &Client{
		apiURL:   apiURL,
		apiToken: apiToken,
		http:     &http.Client{},
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

func (c *Client) SpawnServer(username, serverName, profile string) error {
	body := map[string]string{"profile": profile}
	resp, err := c.do(http.MethodPost, "/users/"+username+"/servers/"+serverName, body)
	if err != nil {
		return fmt.Errorf("spawning server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("spawn failed (%d): %s", resp.StatusCode, b)
	}
	return nil
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
