// Package client provides an HTTP client for the oasis management API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIError represents a non-2xx response from the management API.
type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

// Error implements the error interface, returning a human-readable description.
func (e *APIError) Error() string {
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// Client wraps an *http.Client for communicating with the oasis management API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	cliVersion string
}

// New returns a Client with a 10-second timeout.
func New(baseURL, cliVersion string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
		cliVersion: cliVersion,
	}
}

// WithTimeout returns a new Client with the specified timeout.
func (c *Client) WithTimeout(d time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: d},
		baseURL:    c.baseURL,
		cliVersion: c.cliVersion,
	}
}

// Get performs a GET request to the given path and decodes the JSON response into out.
func (c *Client) Get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	return c.do(req, out)
}

// Post performs a POST request with a JSON body and decodes the response into out (out may be nil).
func (c *Client) Post(path string, body, out interface{}) error {
	return c.doWithBody(http.MethodPost, path, body, out)
}

// Patch performs a PATCH request with a JSON body and decodes the response into out (out may be nil).
func (c *Client) Patch(path string, body, out interface{}) error {
	return c.doWithBody(http.MethodPatch, path, body, out)
}

// Delete performs a DELETE request; expects a 204 No Content response.
func (c *Client) Delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	return c.do(req, nil)
}

func (c *Client) doWithBody(method, path string, body, out interface{}) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}

	req, err := http.NewRequest(method, c.baseURL+path, &buf)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	return c.do(req, out)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Oasis-CLI-Version", c.cliVersion)
}

func (c *Client) do(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.decodeError(resp)
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	}
	// Drain body to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) decodeError(resp *http.Response) error {
	var payload struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)

	msg := payload.Error
	if msg == "" {
		msg = resp.Status
	}
	code := payload.Code
	if code == "" {
		code = "UNKNOWN"
	}

	return &APIError{
		Code:       code,
		Message:    msg,
		HTTPStatus: resp.StatusCode,
	}
}
