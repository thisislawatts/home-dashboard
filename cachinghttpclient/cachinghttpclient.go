// CachingHTTPClient wraps http.Client and caches responses for 60 seconds
// Use sync.RWMutex for concurrent reads
package cachinghttpclient

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"
)

type CachingHTTPClient struct {
	Client *http.Client
	Cache  map[string]CacheEntry
	Mutex  sync.RWMutex
	TTL    time.Duration
}

func NewCachingHTTPClient(ttl time.Duration) *CachingHTTPClient {
	return &CachingHTTPClient{
		Client: &http.Client{},
		Cache:  make(map[string]CacheEntry),
		TTL:    ttl,
	}
}

func (c *CachingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	cacheKey := req.Method + ":" + req.URL.String()
	c.Mutex.RLock()
	entry, found := c.Cache[cacheKey]
	c.Mutex.RUnlock()
	if found && time.Now().Before(entry.Expires) {
		return &http.Response{
			StatusCode: entry.Status,
			Body:       io.NopCloser(bytes.NewReader(entry.Response)),
			Header:     entry.Header.Clone(),
		}, nil
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.Mutex.Lock()
	c.Cache[cacheKey] = CacheEntry{
		Response: body,
		Expires:  time.Now().Add(c.TTL),
		Status:   resp.StatusCode,
		Header:   resp.Header.Clone(),
	}
	c.Mutex.Unlock()

	return &http.Response{
		StatusCode: resp.StatusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     resp.Header.Clone(),
	}, nil
}
