package api

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/event"
	"go_base_skeleton/internal/repository"
	"go_base_skeleton/internal/service"
)

// Handler 聚合 API 处理器所需的基础设施依赖与示例服务。
// 新增领域时只需在此处追加 repo→service 接线，app 层无需改动。
type Handler struct {
	DB        *gorm.DB
	Redis     *redis.Client
	Publisher *event.Publisher
	UserSvc   *service.UserService
	AppCfg    config.AppConfig // 应用配置
}

// NewHandler 构造 Handler，接收基础设施依赖，内部完成 repository→service 组装。
func NewHandler(db *gorm.DB, rdb *redis.Client, cfg config.Config, publisher *event.Publisher) *Handler {
	userRepo := repository.NewUserRepository(db)

	userSvc := service.NewUserService(userRepo)

	return &Handler{
		DB:        db,
		Redis:     rdb,
		Publisher: publisher,
		UserSvc:   userSvc,
		AppCfg:    cfg.App,
	}
}
