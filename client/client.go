package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/rs/zerolog/log"

	hstats "github.com/michele/hourly-stats"
)

var (
	ErrNoAPIToken = errors.New("no API token was set")

	// Clickup API errors
	ErrBadRequest   = errors.New("server returned bad request error")
	ErrUnauthorized = errors.New("server returned unauthorized error (token might be invalid)")
	ErrForbidden    = errors.New("server returned forbidden error")
	ErrNotFound     = errors.New("server returned not found error")
	ErrConflict     = errors.New("server returned conflict error")
	ErrTooMany      = errors.New("server returned too many requests error")
	ErrInternal     = errors.New("server returned internal error")
	ErrUnavailable  = errors.New("server is unavailable")
)

type Client struct {
	http    *http.Client
	baseURL *url.URL
	token   string
	retry   int
}

func New(hstatsURL, token string) (*Client, error) {
	baseURL, err := url.Parse(hstatsURL)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		MaxIdleConns:        100,
		TLSHandshakeTimeout: 20 * time.Second,
		MaxConnsPerHost:     100,
		MaxIdleConnsPerHost: 100,
	}
	return &Client{
		http: &http.Client{
			Transport: tr,
		},
		baseURL: baseURL,
		token:   token,
	}, nil
}

func (c *Client) do(method string, endp string, obj interface{}) ([]byte, error) {
	if len(c.token) == 0 {
		return nil, ErrNoAPIToken
	}
	for i := c.retry; i >= 0; i-- {
		var body io.Reader
		if obj != nil && method != "GET" {
			bts, err := json.Marshal(obj)
			if err != nil {
				return nil, err
			}
			body = bytes.NewBuffer(bts)
		}

		endURL := *(c.baseURL)
		endURL.Path = path.Join(endURL.Path, endp)
		if obj != nil && method == "GET" {
			params, ok := obj.(url.Values)
			if ok {
				endURL.RawQuery = params.Encode()
			}
		}
		req, err := http.NewRequest(method, endURL.String(), body)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", c.token)
		resp, err := c.http.Do(req)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			continue
		}
		bts, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return bts, nil
		}
		log.Printf("Error: %+v\n", *resp)
		switch resp.StatusCode {
		case 400:
			return nil, ErrBadRequest
		case 401:
			return nil, ErrUnauthorized
		case 403:
			return nil, ErrForbidden
		case 404:
			return nil, ErrNotFound
		case 409:
			return nil, ErrConflict
		case 429:
			return nil, ErrTooMany
		case 500:
			if i == 0 {
				return nil, ErrInternal
			}
		case 503:
			if i == 0 {
				return nil, ErrUnavailable
			}
		}
	}
	return nil, errors.New("got HTTP error")
}

func (c *Client) Incr(ctx context.Context, bucket, value string) error {
	_, err := c.do("POST", fmt.Sprintf("/stats/%s/%s", bucket, value), nil)
	return err
}

func (c *Client) Report(ctx context.Context, bucket string) (*hstats.Report, error) {
	bts, err := c.do("GET", fmt.Sprintf("/stats/%s", bucket), nil)
	if err != nil {
		return nil, err
	}
	var report hstats.Report
	err = json.Unmarshal(bts, &report)
	if err != nil {
		return nil, err
	}
	return &report, nil
}
