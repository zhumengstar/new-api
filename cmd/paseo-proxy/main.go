package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type endpoint struct {
	raw       string
	target    *url.URL
	apiKey    string
	failCount int32
}

type balancer struct {
	mu        sync.Mutex
	items     []*endpoint
	active    int
	threshold int32
}

func main() {
	listenAddr := getEnv("PASEO_LISTEN", "127.0.0.1:8787")
	rawURLs := getEnv("PASEO_URLS", "")
	rawKeys := getEnv("PASEO_KEYS", "")
	if rawURLs == "" || rawKeys == "" {
		log.Fatal("PASEO_URLS and PASEO_KEYS are required")
	}

	urls := splitAndTrim(rawURLs)
	keys := splitAndTrim(rawKeys)
	if len(urls) == 0 || len(keys) == 0 {
		log.Fatal("PASEO_URLS and PASEO_KEYS must not be empty")
	}
	if len(keys) != 1 && len(keys) != len(urls) {
		log.Fatal("PASEO_KEYS must have either 1 item or match PASEO_URLS count")
	}

	endpoints := make([]*endpoint, 0, len(urls))
	for i, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			log.Fatalf("invalid PASEO_URLS entry: %q", raw)
		}
		key := keys[0]
		if len(keys) > 1 {
			key = keys[i]
		}
		endpoints = append(endpoints, &endpoint{raw: raw, target: u, apiKey: key})
	}

	b := &balancer{
		items:     endpoints,
		threshold: int32(getEnvInt("PASEO_FAIL_THRESHOLD", 3)),
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout:  60 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	if proxyURL := strings.TrimSpace(os.Getenv("PASEO_PROXY")); proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		up, idx, err := b.next()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(up.target)
		proxy.Transport = transport
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = up.target.Scheme
			req.URL.Host = up.target.Host
			req.Host = up.target.Host
			req.URL.Path = joinPath(up.target.Path, r.URL.Path)
			req.URL.RawPath = joinPath(up.target.Path, r.URL.RawPath)
			req.URL.RawQuery = r.URL.RawQuery
			req.Header = cloneHeader(r.Header)
			req.Header.Del("Accept-Encoding")
			req.Header.Set("Authorization", "Bearer "+up.apiKey)
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
			b.recordFailure(idx)
			http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode >= 500 {
				b.recordFailure(idx)
				return nil
			}
			b.recordSuccess(idx)
			return nil
		}
		proxy.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("paseo proxy listening on %s", listenAddr)
	log.Fatal(srv.ListenAndServe())
}

func (b *balancer) next() (*endpoint, int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.items) == 0 {
		return nil, 0, errors.New("no upstreams configured")
	}

	start := b.active
	for i := 0; i < len(b.items); i++ {
		idx := (start + i) % len(b.items)
		if atomic.LoadInt32(&b.items[idx].failCount) < b.threshold {
			b.active = idx
			return b.items[idx], idx, nil
		}
	}

	return nil, 0, errors.New("all upstreams are unavailable")
}

func (b *balancer) recordFailure(idx int) {
	up := b.items[idx]
	count := atomic.AddInt32(&up.failCount, 1)
	if count < b.threshold {
		return
	}

	atomic.StoreInt32(&up.failCount, 0)
	b.mu.Lock()
	b.active = (idx + 1) % len(b.items)
	b.mu.Unlock()
}

func (b *balancer) recordSuccess(idx int) {
	atomic.StoreInt32(&b.items[idx].failCount, 0)
}

func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getEnv(key, def string) string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	return raw
}

func getEnvInt(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
		return def
	}
	return n
}

func joinPath(basePath, reqPath string) string {
	basePath = strings.TrimRight(basePath, "/")
	if reqPath == "" {
		return basePath
	}
	if strings.HasPrefix(reqPath, "/") {
		return basePath + reqPath
	}
	return basePath + "/" + reqPath
}

func cloneHeader(src http.Header) http.Header {
	dst := make(http.Header, len(src))
	for k, vv := range src {
		dst[k] = append([]string(nil), vv...)
	}
	return dst
}
