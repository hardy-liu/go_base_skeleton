package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/errcode"
	"go_base_skeleton/internal/pkg/response"
)

const (
	// CtxKeyClaims 在 gin.Context 中存放 jwt.MapClaims。
	CtxKeyClaims = "claims"
	// CtxKeyUID 当 token 的 claims 含 uid 时写入，便于 handler 读取。
	CtxKeyUID = "uid"
)

// JWT 校验 Authorization: Bearer <token>：使用 cfg.Secret、校验 issuer 与过期时间（jwt.WithExpirationRequired）。
// 校验通过后写入 claims；若 claims["uid"] 存在则同时写入 CtxKeyUID。
func JWT(cfg config.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, errcode.ErrUnauthorized)
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			return []byte(cfg.Secret), nil
		},
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			response.Fail(c, errcode.ErrUnauthorized.WithMsg("invalid or expired token"))
			c.Abort()
			return
		}

		c.Set(CtxKeyClaims, claims)
		if uid, ok := claims["uid"]; ok {
			c.Set(CtxKeyUID, uid)
		}

		c.Next()
	}
}
