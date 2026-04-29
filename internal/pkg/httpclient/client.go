package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"

	"go_base_skeleton/internal/pkg/util"
)

type Client interface {
	Get(ctx context.Context, url string, query map[string]string, result any, opts ...Option) error
	Post(ctx context.Context, url string, body any, result any, opts ...Option) error
	PostForm(ctx context.Context, url string, formData map[string]string, result any, opts ...Option) error
}

type Config struct {
	Timeout             time.Duration
	RetryCount          int
	RetryWaitTime       time.Duration
	RetryMaxWaitTime    time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

var DefaultConfig = Config{
	Timeout:          5 * time.Second, //单次http超时时间
	RetryCount:       1,
	RetryWaitTime:    200 * time.Millisecond,
	RetryMaxWaitTime: 1 * time.Second,

	MaxIdleConns:        50,
	MaxIdleConnsPerHost: 20,
	IdleConnTimeout:     90 * time.Second,
}

// WithDefaults 用默认配置补齐当前 Config 中的零值字段，便于只覆盖少数字段时保持其余参数稳定。
// 传参Config里面有值的，就覆盖默认配置的值
func (c Config) WithDefaults() Config {
	out := DefaultConfig

	if c.Timeout > 0 {
		out.Timeout = c.Timeout
	}
	if c.RetryCount != 0 {
		out.RetryCount = c.RetryCount
	}
	if c.RetryWaitTime > 0 {
		out.RetryWaitTime = c.RetryWaitTime
	}
	if c.RetryMaxWaitTime > 0 {
		out.RetryMaxWaitTime = c.RetryMaxWaitTime
	}
	if c.MaxIdleConns > 0 {
		out.MaxIdleConns = c.MaxIdleConns
	}
	if c.MaxIdleConnsPerHost > 0 {
		out.MaxIdleConnsPerHost = c.MaxIdleConnsPerHost
	}
	if c.IdleConnTimeout > 0 {
		out.IdleConnTimeout = c.IdleConnTimeout
	}

	return out
}

type options struct {
	headers map[string]string
}

// 接收 options 作为参数，并修改它
type Option func(*options)

func applyOptions(opts ...Option) options {
	o := options{
		headers: make(map[string]string),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func WithHeader(k, v string) Option {
	return func(o *options) {
		o.headers[k] = v
	}
}

type RestyClient struct {
	client *resty.Client
	log    *zap.Logger
}

func NewRestyClient(cfg Config, log *zap.Logger, retryCondition resty.RetryConditionFunc) *RestyClient {
	if log == nil {
		log = zap.NewNop()
	}
	if retryCondition == nil {
		retryCondition = defaultRetryConditionFunc
	}
	cfg = cfg.WithDefaults()

	r := resty.New()
	// 基础配置
	r.SetTimeout(cfg.Timeout).
		SetRetryCount(cfg.RetryCount).
		SetRetryWaitTime(cfg.RetryWaitTime).
		SetRetryMaxWaitTime(cfg.RetryMaxWaitTime)

	// 连接池
	tr := http.DefaultTransport.(*http.Transport).Clone() //先继承默认配置，然后再覆盖
	tr.MaxIdleConns = cfg.MaxIdleConns
	tr.MaxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	tr.IdleConnTimeout = cfg.IdleConnTimeout
	tr.TLSHandshakeTimeout = cfg.Timeout
	r.SetTransport(tr)

	// retry 策略
	r.AddRetryCondition(retryCondition)

	client := &RestyClient{
		client: r,
		log:    log,
	}
	client.registerHooks()
	return client
}

// 重试机制需要谨慎，不然下游不做幂等容易引发重复提交
func defaultRetryConditionFunc(resp *resty.Response, err error) bool {
	if err != nil { //经测试ctx的 timeout 没进来这里
		// fmt.Printf("err detail: %+v\n", err)
		if errors.Is(err, context.Canceled) {
			return false
		}
		if errors.Is(err, syscall.ECONNREFUSED) ||
			errors.Is(err, syscall.EHOSTUNREACH) ||
			errors.Is(err, syscall.ENETUNREACH) {
			return false
		}

		var netErr *net.OpError
		if errors.As(err, &netErr) { //网络错误
			if netErr.Op == "dial" { //端口被拒绝
				return false
			}
		}
		// 兜底：字符串匹配（简单可靠）
		s := err.Error()
		if strings.Contains(s, "connection refused") || strings.Contains(s, "reset by peer") {
			return false
		}

		//http 的 timeout 会触发这里，需要进行重试，经测试 ctx的timeout没进来这里
		if errors.Is(err, context.DeadlineExceeded) {
			return true
		}
	}
	return resp.StatusCode() >= 500
}

// 加上请求的 req queyr body 和响应的req query 和 resp body
func (c *RestyClient) registerHooks() {
	c.client.OnBeforeRequest(func(cli *resty.Client, req *resty.Request) error {
		c.log.Info("http request",
			zap.String("method", req.Method),
			zap.String("url", req.URL),
			zap.Int("req_attemp", req.Attempt),
			zap.String("req_query", req.QueryParam.Encode()),
			zap.String("req_body", formatReqBody(req.Body)),
			zap.String("req_form_data", formatReqBody(req.FormData)),
		)
		return nil
	})

	c.client.OnAfterResponse(func(cli *resty.Client, resp *resty.Response) error {
		c.log.Info("http response",
			zap.Int("status", resp.StatusCode()),
			zap.String("url", resp.Request.URL),
			zap.Duration("cost", resp.Time()),
			zap.String("resp_body", string(util.TruncateUTF8(resp.Body(), truncateMaxBytes))),
		)
		return nil
	})
}

// Get 和 Post 方法都可以执行执行请求级别的头部，通过传多个 Option 函数参数来实现（请求级别的timeout通过带timeout的ctx来实现）
func (c *RestyClient) Get(ctx context.Context, url string, query map[string]string, result any, opts ...Option) error {
	if err := util.EnsurePointer(result); err != nil {
		return err
	}

	o := applyOptions(opts...)

	req := c.client.R().
		SetContext(ctx).
		SetResult(result)
	if query != nil {
		req.SetQueryParams(query)
	}

	for k, v := range o.headers {
		req.SetHeader(k, v)
	}

	resp, err := req.Get(url)
	if err != nil {
		return err
	}

	if resp.IsError() {
		return buildHTTPError(resp)
	}

	return nil
}

// POST JSON，会带上 Content-Type: application/json 头部
func (c *RestyClient) Post(ctx context.Context, url string, body any, result any, opts ...Option) error {
	if err := util.EnsurePointer(result); err != nil {
		return err
	}

	o := applyOptions(opts...)

	req := c.client.R().
		SetContext(ctx).
		SetBody(body).
		SetResult(result)

	for k, v := range o.headers {
		req.SetHeader(k, v)
	}

	resp, err := req.Post(url)
	if err != nil {
		return err
	}

	if resp.IsError() {
		return buildHTTPError(resp)
	}

	return nil
}

// POST表单，会带上 Content-Type: application/x-www-form-urlencoded 头部
func (c *RestyClient) PostForm(ctx context.Context, url string, formData map[string]string, result any, opts ...Option) error {
	if err := util.EnsurePointer(result); err != nil {
		return err
	}

	o := applyOptions(opts...)

	req := c.client.R().
		SetContext(ctx).
		SetFormData(formData).
		SetResult(result)

	for k, v := range o.headers {
		req.SetHeader(k, v)
	}

	resp, err := req.Post(url)
	if err != nil {
		return err
	}

	if resp.IsError() {
		return buildHTTPError(resp)
	}

	return nil
}

var truncateMaxBytes = 4096

func formatReqBody(body any) string {
	if body == nil {
		return ""
	}

	switch v := body.(type) {
	case []byte:
		return string(util.TruncateUTF8(v, truncateMaxBytes))
	case string:
		return string(util.TruncateUTF8([]byte(v), truncateMaxBytes))
	case fmt.Stringer:
		return v.String()
	case io.Reader:
		return "[stream body omitted]"
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("[marshal error: %v]", err)
		}
		return string(util.TruncateUTF8(b, truncateMaxBytes))
	}
}

func buildHTTPError(resp *resty.Response) error {
	return fmt.Errorf("http error: status=%d url=%s body=%s",
		resp.StatusCode(),
		resp.Request.URL,
		string(util.TruncateUTF8(resp.Body(), truncateMaxBytes)),
	)
}
