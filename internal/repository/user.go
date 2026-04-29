package repository

import (
	"context"

	"gorm.io/gorm"

	"go_base_skeleton/internal/model"
)

// UserRepository 封装 user 表的 GORM 访问；db 为 nil 时调用方会 panic（测试或错误注入场景除外）。
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建仓储实例。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByUID 按主键 uid 查询一行；不存在时返回 gorm.ErrRecordNotFound，由 service 映射为业务错误。
func (r *UserRepository) FindByUID(ctx context.Context, uid int64) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).Where("uid = ?", uid).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}
