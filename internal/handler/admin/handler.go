package admin

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"go_base_skeleton/internal/repository"
	"go_base_skeleton/internal/service"
)

// Handler 聚合管理后台 API 处理器依赖（与业务 API 结构对称，便于独立进程部署）。
// 新增领域时只需在此处追加 repo→service 接线，app 层无需改动。
type Handler struct {
	DB      *gorm.DB
	Redis   *redis.Client
	UserSvc *service.UserService
}

// NewHandler 构造 Admin Handler, 接收基础设施依赖，内部完成 repository→service 组装。
func NewHandler(db *gorm.DB, rdb *redis.Client) *Handler {
	userRepo := repository.NewUserRepository(db)
	userSvc := service.NewUserService(userRepo)

	return &Handler{
		DB:      db,
		Redis:   rdb,
		UserSvc: userSvc,
	}
}
