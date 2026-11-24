// Package httpx 带指数退避重试的 HTTP 下载，429/5xx 与网络错误自动重试。
package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// StatusError 携带 HTTP 状态码的错误
type StatusError struct{ Status int }

func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d", e.Status) }

// Get 下载 url 全部响应体。
// retries 为最大重试次数（0 = 单次尝试）；429/5xx 与网络错误按指数退避重试，
// 其余错误（3xx/4xx）立即失败。返回的 StatusError 可用 errors.As 取状态码。
func Get(ctx context.Context, client *http.Client, url string, retries int) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return Do(ctx, client, req, retries)
}

// Do 执行 req 并返回全部响应体（重试语义同 Get）。
// 每次尝试克隆请求并重放请求体，req 本身不被修改。
// 仅幂等方法（GET/HEAD/OPTIONS/PUT/DELETE）自动重试，避免重放副作用。
func Do(ctx context.Context, client *http.Client, req *http.Request, retries int) ([]byte, error) {
	var lastErr error
	for attempt := range retries + 1 {
		if attempt > 0 {
			// 指数退避 + 随机抖动，避免重试风暴
			backoff := time.Duration(1<<uint(attempt-1))*2*time.Second + time.Duration(rand.IntN(1000))*time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		body, err := do(ctx, client, req)
		if err == nil {
			return body, nil
		}
		lastErr = err
		// 非幂等方法不重试；请求体无法重放时不重试（避免重试发送空 body 造成静默截断）；其余按 retryable 判定
		if !idempotent(req.Method) || !canReplay(req) || !retryable(err) {
			break
		}
	}
	return nil, fmt.Errorf("下载失败 %s: %w", req.URL, lastErr)
}

// canReplay 请求体可重放：无 body，或 GetBody 可重建 body
func canReplay(req *http.Request) bool {
	return req.Body == nil || req.GetBody != nil
}

// idempotent 可安全自动重试的 HTTP 方法
func idempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete, http.MethodTrace:
		return true
	}
	return false
}

// do 单次请求（克隆 req，避免重试间相互污染）
func do(ctx context.Context, client *http.Client, req *http.Request) ([]byte, error) {
	clone := req.Clone(ctx)
	// 重放请求体：Clone 共享同一 Body 读取器，每次尝试须重置
	if clone.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
	}
	resp, err := client.Do(clone)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Status: resp.StatusCode}
	}
	return io.ReadAll(resp.Body)
}

// retryable 429 与 5xx 可重试；4xx 其他错误不可重试；网络错误默认可重试
func retryable(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status == http.StatusTooManyRequests || se.Status >= 500
	}
	return true
}
