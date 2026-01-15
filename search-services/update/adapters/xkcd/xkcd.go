package xkcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"yadro.com/course/update/core"
)

type Client struct {
	log     *slog.Logger
	baseURL string
	timeout time.Duration
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration, log *slog.Logger) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("empty base url")
	}
	return &Client{
		log:     log,
		baseURL: baseURL,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
	}, nil
}

type comicResp struct {
	Num        int    `json:"num"`
	Img        string `json:"img"`
	Title      string `json:"safe_title"`
	Transcript string `json:"transcript"`
	Alt        string `json:"alt"`
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
		} else {
			func() {
				defer func() {
					if cerr := resp.Body.Close(); cerr != nil {
						c.log.Warn("close response body failed", "error", cerr)
					}
				}()
				if resp.StatusCode == http.StatusNotFound {
					lastErr = core.ErrNotFound
					return
				}
				if resp.StatusCode != http.StatusOK {
					if resp.StatusCode >= 500 {
						lastErr = fmt.Errorf("unexpected status: %s", resp.Status)
						return
					}
					lastErr = fmt.Errorf("unexpected status: %s", resp.Status)
					attempt = 2
					return
				}
				if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
					lastErr = err
					return
				}
				lastErr = nil
			}()
			if lastErr == nil {
				return nil
			}
			if errors.Is(lastErr, core.ErrNotFound) {
				return lastErr
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	return lastErr
}

func (c *Client) Get(ctx context.Context, id int) (core.XKCDInfo, error) {
	var cr comicResp
	if err := c.getJSON(ctx, fmt.Sprintf("%s/%d/info.0.json", c.baseURL, id), &cr); err != nil {
		return core.XKCDInfo{}, err
	}
	return core.XKCDInfo{
		ID:          cr.Num,
		URL:         cr.Img,
		Title:       cr.Title,
		Description: cr.Alt + " " + cr.Transcript,
	}, nil
}

func (c *Client) LastID(ctx context.Context) (int, error) {
	var cr comicResp
	if err := c.getJSON(ctx, fmt.Sprintf("%s/info.0.json", c.baseURL), &cr); err != nil {
		return 0, err
	}
	return cr.Num, nil
}
