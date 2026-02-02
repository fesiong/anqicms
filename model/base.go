package model

const (
	StatusWait = uint(0)
	StatusOk   = uint(1)
)

type CustomField struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	Default     interface{} `json:"default"`
	FollowLevel bool        `json:"follow_level"`
	Type        string      `json:"-"`
	FieldName   string      `json:"-"`
}

type CustomFieldTexts struct {
	Key    string   `json:"key"`
	Value  string   `json:"value"`
	Values []string `json:"values"` // 更多的字段
}

type TimelineField struct {
	Title   string            `json:"title"`
	Content string            `json:"content"`
	Status  string            `json:"status"`
	Images  []string          `json:"images"`
	Extra   map[string]string `json:"extra"`
	Items   []TimelineField   `json:"items,omitempty"`
}
