package config

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type MysqlConfig struct {
	Database   string `json:"database"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	UseDefault bool   `json:"use_default"` // 使用 default 的账号密码
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
