package service

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"go_base_skeleton/internal/model"
	"go_base_skeleton/internal/pkg/errcode"
	"go_base_skeleton/internal/pkg/logger"
	"go_base_skeleton/internal/repository"
)

// UserService 用户领域服务：编排日志、仓储调用与错误码转换。
type UserService struct {
	repo *repository.UserRepository
}

// NewUserService 构造服务。
func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// GetByUID 根据 UID 查询用户：记录入口日志；无记录返回 ErrUserNotFound；其他 DB 错误记日志并返回 ErrDatabase。
func (s *UserService) GetByUID(ctx context.Context, uid int64) (*model.User, error) {
	logger.WithCtx(ctx).Info("UserService.GetByUID called", zap.Int64("uid", uid))

	u, err := s.repo.FindByUID(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.ErrUserNotFound
		}
		logger.WithCtx(ctx).Error("UserService.GetByUID db error", zap.Error(err))
		return nil, errcode.ErrDatabase
	}
	return u, nil
}
