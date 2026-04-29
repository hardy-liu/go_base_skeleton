package api

import (
	"bytes"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/pkg/response"
)

// DebugHeaders 处理 GET/POST /debug：用结构化 logger 各一行记录 header、query、body。
func (h *Handler) DebugHeaders(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err == nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}

	var hdrBuf bytes.Buffer
	_ = c.Request.Header.Write(&hdrBuf)
	hdrLine := strings.ReplaceAll(strings.TrimSpace(hdrBuf.String()), "\r\n", "\n")

	l := logger.WithCtx(c.Request.Context())
	l.Info("url", zap.String("dump", c.Request.URL.RawPath))
	l.Info("headers", zap.String("dump", hdrLine))
	l.Info("query", zap.String("dump", c.Request.URL.RawQuery))
	if err != nil {
		l.Info("body", zap.String("dump", ""), zap.Error(err))
	} else if len(body) == 0 {
		l.Info("body", zap.String("dump", ""))
	} else {
		l.Info("body", zap.String("dump", string(body)))
	}

	if rand.Intn(2) < 1 {
		time.Sleep(6 * time.Second)
	}
	response.OK(c, nil)
}
