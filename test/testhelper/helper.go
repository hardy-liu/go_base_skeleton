package testhelper

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go_base_skeleton/internal/pkg/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// NewTestRouter 返回未挂载默认中间件的 gin 引擎，便于测试按需叠加中间件。
func NewTestRouter() *gin.Engine {
	return gin.New()
}

// PerformRequest 构造无 Body 的请求并执行 ServeHTTP，返回 ResponseRecorder 供断言。
func PerformRequest(e *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	e.ServeHTTP(w, req)
	return w
}

// DecodeBody 将响应体反序列化为 response.Body 结构体
func DecodeBody(t *testing.T, w *httptest.ResponseRecorder) response.Body {
	t.Helper()
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var b response.Body
	if err := json.Unmarshal(body, &b); err != nil {
		t.Fatalf("unmarshal body %q: %v", string(body), err)
	}
	return b
}

// AssertStatus 断言 HTTP 状态码。
func AssertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("expected status %d, got %d (body: %s)", expected, w.Code, w.Body.String())
	}
}

// AssertCode 断言响应 JSON 中的业务 code 字段。
func AssertCode(t *testing.T, w *httptest.ResponseRecorder, expectedCode int) {
	t.Helper()
	b := DecodeBody(t, w)
	if b.Code != expectedCode {
		t.Errorf("expected code %d, got %d (message: %s)", expectedCode, b.Code, b.Message)
	}
}
