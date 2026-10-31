package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// sanitize makes a Drive or local name safe as a path component.
func sanitize(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '/' || r == '\\' || r == 0 || r < 32 {
			out = append(out, '_')
		} else {
			out = append(out, r)
		}
	}
	s := string(out)
	s = trimDots(s)
	if s == "" {
		return "unnamed"
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func trimDots(s string) string {
	for len(s) > 0 && (s[0] == '.' || s[0] == ' ') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == '.' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}

func humanSize(n int64) string {
	if n < 0 {
		return "?"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// retry runs fn with exponential backoff on transient HTTP errors.
func retry(fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		resp, err := fn()
		if err == nil && resp.StatusCode < 400 {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
			code := resp.StatusCode
			lastErr = fmt.Errorf("HTTP %d", code)
			if code != 429 && code < 500 {
				// Non-transient: caller needs the body for the message.
				resp2, err2 := fn()
				return resp2, err2
			}
		} else {
			lastErr = err
		}
		time.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return nil, lastErr
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
