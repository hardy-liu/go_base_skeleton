package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stringerStub string

func (s stringerStub) String() string {
	return string(s)
}

type readerStub struct {
	*strings.Reader
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDefaultRetryConditionFunc(t *testing.T) {
	tests := []struct {
		name string
		resp *resty.Response
		err  error
		want bool
	}{
		{
			name: "context canceled should not retry",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "connection refused should not retry",
			err:  syscall.ECONNREFUSED,
			want: false,
		},
		{
			name: "dial net error should not retry",
			err: &net.OpError{
				Op:  "dial",
				Err: errors.New("dial failed"),
			},
			want: false,
		},
		{
			name: "connection reset text should not retry",
			err:  errors.New("read: connection reset by peer"),
			want: false,
		},
		{
			name: "deadline exceeded should retry",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "server error should retry",
			resp: &resty.Response{
				RawResponse: &http.Response{StatusCode: http.StatusInternalServerError},
			},
			want: true,
		},
		{
			name: "client error should not retry",
			resp: &resty.Response{
				RawResponse: &http.Response{StatusCode: http.StatusBadRequest},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, defaultRetryConditionFunc(tt.resp, tt.err))
		})
	}
}

func TestFormatReqBody(t *testing.T) {
	orig := truncateMaxBytes
	// 缩小截断阈值，方便验证日志内容是否按字节安全裁剪。
	truncateMaxBytes = 5
	defer func() {
		truncateMaxBytes = orig
	}()

	tests := []struct {
		name string
		body any
		want string
	}{
		{
			name: "nil body",
			body: nil,
			want: "",
		},
		{
			name: "byte slice body",
			body: []byte("ab中cd"),
			want: "ab中",
		},
		{
			name: "string body",
			body: "ab中cd",
			want: "ab中",
		},
		{
			name: "stringer body",
			body: stringerStub("stringer"),
			want: "stringer",
		},
		{
			name: "reader body",
			body: readerStub{Reader: strings.NewReader("stream")},
			want: "[stream body omitted]",
		},
		{
			name: "marshal struct body",
			body: map[string]any{"k": "v"},
			want: "{\"k\":",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 覆盖 formatReqBody 的主要分支，确保日志格式稳定。
			assert.Equal(t, tt.want, formatReqBody(tt.body))
		})
	}

	t.Run("marshal error", func(t *testing.T) {
		// 函数类型不可被 json.Marshal，应该返回可观测的错误提示文本。
		got := formatReqBody(func() {})
		assert.Contains(t, got, "[marshal error:")
	})
}

func TestBuildHTTPError(t *testing.T) {
	orig := truncateMaxBytes
	// 缩小阈值，验证错误消息里的 body 也会走 UTF-8 安全截断。
	truncateMaxBytes = 5
	defer func() {
		truncateMaxBytes = orig
	}()

	resp := &resty.Response{
		Request: &resty.Request{URL: "http://example.com/test"},
		RawResponse: &http.Response{
			StatusCode: http.StatusBadGateway,
		},
	}
	resp.SetBody([]byte("ab中cd"))

	err := buildHTTPError(resp)

	require.Error(t, err)
	assert.Equal(t, "http error: status=502 url=http://example.com/test body=ab中", err.Error())
}

func TestRestyClientGet(t *testing.T) {
	t.Run("success with query and headers", func(t *testing.T) {
		client := NewRestyClient(DefaultConfig, nil, nil)
		client.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
			// 这里直接检查最终发出的请求，避免测试只验证返回值却漏掉请求拼装。
			assert.Equal(t, "v1", r.URL.Query().Get("q"))
			assert.Equal(t, "token", r.Header.Get("X-Test"))
			assert.Equal(t, http.MethodGet, r.Method)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true,"message":"hello"}`)),
				Request:    r,
			}, nil
		}))
		var result struct {
			OK      bool   `json:"ok"`
			Message string `json:"message"`
		}

		err := client.Get(context.Background(), "http://example.com/path", map[string]string{"q": "v1"}, &result, WithHeader("X-Test", "token"))

		require.NoError(t, err)
		assert.True(t, result.OK)
		assert.Equal(t, "hello", result.Message)
	})

	t.Run("non pointer result returns error", func(t *testing.T) {
		client := NewRestyClient(DefaultConfig, nil, nil)
		var result struct{}

		err := client.Get(context.Background(), "http://example.com", nil, result)

		require.Error(t, err)
		assert.Equal(t, "result must be a pointer", err.Error())
	})

	t.Run("http error returns wrapped error", func(t *testing.T) {
		client := NewRestyClient(DefaultConfig, nil, nil)
		client.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
			// 构造 5xx 响应，验证调用层拿到的是统一包装后的错误文本。
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("boom")),
				Request:    r,
			}, nil
		}))
		var result struct{}
		targetURL := "http://example.com/fail"

		err := client.Get(context.Background(), targetURL, nil, &result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "http error: status=500")
		assert.Contains(t, err.Error(), targetURL)
		assert.Contains(t, err.Error(), "body=boom")
	})
}

func TestRestyClientPost(t *testing.T) {
	t.Run("success with headers and body", func(t *testing.T) {
		client := NewRestyClient(DefaultConfig, nil, nil)
		client.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
			// 校验 Post 是否把 header 和 JSON body 都正确发出。
			assert.Equal(t, "token", r.Header.Get("X-Test"))
			assert.Equal(t, http.MethodPost, r.Method)

			var req struct {
				Name string `json:"name"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "alice", req.Name)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Request:    r,
			}, nil
		}))
		var result struct {
			OK bool `json:"ok"`
		}

		err := client.Post(context.Background(), "http://example.com/path", map[string]string{"name": "alice"}, &result, WithHeader("X-Test", "token"))

		require.NoError(t, err)
		assert.True(t, result.OK)
	})

	t.Run("non pointer result returns error", func(t *testing.T) {
		client := NewRestyClient(DefaultConfig, nil, nil)
		var result struct{}

		err := client.Post(context.Background(), "http://example.com", map[string]string{"k": "v"}, result)

		require.Error(t, err)
		assert.Equal(t, "result must be a pointer", err.Error())
	})
}

func TestRestyClientPostForm(t *testing.T) {
	t.Run("success with form data", func(t *testing.T) {
		client := NewRestyClient(DefaultConfig, nil, nil)
		client.client.SetTransport(roundTripFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			values, err := url.ParseQuery(string(raw))
			require.NoError(t, err)
			assert.Equal(t, "alice", values.Get("name"))
			assert.Equal(t, "18", values.Get("age"))

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Request:    r,
			}, nil
		}))
		var result struct {
			OK bool `json:"ok"`
		}

		err := client.PostForm(context.Background(), "http://example.com/form", map[string]string{
			"name": "alice",
			"age":  "18",
		}, &result)

		require.NoError(t, err)
		assert.True(t, result.OK)
	})

	t.Run("non pointer result returns error", func(t *testing.T) {
		client := NewRestyClient(DefaultConfig, nil, nil)
		var result struct{}

		err := client.PostForm(context.Background(), "http://example.com/form", map[string]string{"name": "alice"}, result)

		require.Error(t, err)
		assert.Equal(t, "result must be a pointer", err.Error())
	})
}

func TestNewRestyClientUsesDefaults(t *testing.T) {
	// 这里只验证构造函数在 nil logger / nil retryCondition 下会补默认值，不测 resty 内部实现。
	client := NewRestyClient(Config{
		Timeout:             time.Second,
		RetryCount:          2,
		RetryWaitTime:       10 * time.Millisecond,
		RetryMaxWaitTime:    20 * time.Millisecond,
		MaxIdleConns:        3,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     30 * time.Second,
	}, nil, nil)

	require.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.NotNil(t, client.log)
}

func TestConfigWithDefaults(t *testing.T) {
	t.Run("fill zero fields from defaults", func(t *testing.T) {
		cfg := Config{
			Timeout: 10 * time.Second,
		}.WithDefaults()

		assert.Equal(t, 10*time.Second, cfg.Timeout)
		assert.Equal(t, DefaultConfig.RetryCount, cfg.RetryCount)
		assert.Equal(t, DefaultConfig.RetryWaitTime, cfg.RetryWaitTime)
		assert.Equal(t, DefaultConfig.RetryMaxWaitTime, cfg.RetryMaxWaitTime)
		assert.Equal(t, DefaultConfig.MaxIdleConns, cfg.MaxIdleConns)
		assert.Equal(t, DefaultConfig.MaxIdleConnsPerHost, cfg.MaxIdleConnsPerHost)
		assert.Equal(t, DefaultConfig.IdleConnTimeout, cfg.IdleConnTimeout)
	})

	t.Run("keep explicit non zero fields", func(t *testing.T) {
		cfg := Config{
			Timeout:             12 * time.Second,
			RetryCount:          3,
			RetryWaitTime:       250 * time.Millisecond,
			RetryMaxWaitTime:    3 * time.Second,
			MaxIdleConns:        99,
			MaxIdleConnsPerHost: 33,
			IdleConnTimeout:     2 * time.Minute,
		}.WithDefaults()

		assert.Equal(t, 12*time.Second, cfg.Timeout)
		assert.Equal(t, 3, cfg.RetryCount)
		assert.Equal(t, 250*time.Millisecond, cfg.RetryWaitTime)
		assert.Equal(t, 3*time.Second, cfg.RetryMaxWaitTime)
		assert.Equal(t, 99, cfg.MaxIdleConns)
		assert.Equal(t, 33, cfg.MaxIdleConnsPerHost)
		assert.Equal(t, 2*time.Minute, cfg.IdleConnTimeout)
	})
}

func TestNewRestyClientAppliesDefaultConfigValues(t *testing.T) {
	client := NewRestyClient(Config{
		Timeout: 8 * time.Second,
	}, nil, nil)

	require.NotNil(t, client)
	assert.Equal(t, 8*time.Second, client.client.GetClient().Timeout)
	assert.Equal(t, DefaultConfig.RetryCount, client.client.RetryCount)
	assert.Equal(t, DefaultConfig.RetryWaitTime, client.client.RetryWaitTime)
	assert.Equal(t, DefaultConfig.RetryMaxWaitTime, client.client.RetryMaxWaitTime)

	tr, ok := client.client.GetClient().Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, DefaultConfig.MaxIdleConns, tr.MaxIdleConns)
	assert.Equal(t, DefaultConfig.MaxIdleConnsPerHost, tr.MaxIdleConnsPerHost)
	assert.Equal(t, DefaultConfig.IdleConnTimeout, tr.IdleConnTimeout)
	assert.Equal(t, 8*time.Second, tr.TLSHandshakeTimeout)
}
