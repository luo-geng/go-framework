package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestGet_RetryOn429 前两次 429、第三次成功：应重试并最终返回内容
func TestGet_RetryOn429(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	body, err := Get(context.Background(), srv.Client(), srv.URL, 3)
	if err != nil {
		t.Fatalf("应重试成功: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("内容不符: %q", body)
	}
	if calls.Load() != 3 {
		t.Fatalf("请求次数不符: %d", calls.Load())
	}
}

// TestGet_4xxNoRetry 404 立即失败，不重试
func TestGet_4xxNoRetry(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Get(context.Background(), srv.Client(), srv.URL, 3)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 404 {
		t.Fatalf("应返回 StatusError 404: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("404 不应重试，实际请求 %d 次", calls.Load())
	}
}

// TestGet_ContextCancel 退避等待期间取消 context
func TestGet_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Get(ctx, srv.Client(), srv.URL, 5)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("应返回 context 超时错误: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("取消后未及时退出")
	}
}
