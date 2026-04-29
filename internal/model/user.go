package model

// User 对应表 user，字段 uid、name；UID 自增主键，与 SPEC 示例一致。
type User struct {
	UID  int64  `gorm:"column:uid;primaryKey;autoIncrement" json:"uid"`
	Name string `gorm:"column:name;type:varchar(128);not null" json:"name"`
}

// TableName 显式指定 GORM 表名为 user（避免复数默认规则）。
func (User) TableName() string {
	return "user"
}
