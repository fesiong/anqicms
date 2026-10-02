package config

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type MysqlConfig struct {
	Database   string `json:"database"`    // 数据库名称
	User       string `json:"user"`        // 数据库用户名
	Password   string `json:"password"`    // 数据库密码
	Host       string `json:"host"`        // 数据库地址， 如：127.0.0.1
	Port       int    `json:"port"`        // 数据库端口，默认：3306
	UseDefault bool   `json:"use_default"` // 复用 默认站点(ID：1) 的数据库账号密码
}

// Value implements the driver.Valuer interface.
func (m MysqlConfig) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface.
func (m *MysqlConfig) Scan(src interface{}) error {
	switch src := src.(type) {
	case []byte:
		return json.Unmarshal(src, &m)
	case string:
		return json.Unmarshal([]byte(src), &m)
	case nil:
		*m = MysqlConfig{}
		return nil
	}

	return fmt.Errorf("pq: cannot convert %T", src)
}
