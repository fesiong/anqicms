package response

import "time"

type AuthResponse struct {
	HashKey     string `json:"hash_key"`
	CreatedTime int64  `json:"created_time"`
	Code        string `json:"code"`
	Status      int    `json:"status"`
}

type SumAmount struct {
	Total int64 `json:"total"`
}

type LoginError struct {
	Times    int
	LastTime int64
}

type FilterRange struct {
	Min   int64  `json:"min"`
	Max   int64  `json:"max"`
	Value string `json:"value"`
	Link  string `json:"link"`
}

type FilterGroup struct {
	Name      string       `json:"name"`
	FieldName string       `json:"field_name"`
	Range     *FilterRange `json:"range,omitempty"`
	Items     []FilterItem `json:"items"`
}

type FilterItem struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	Color     string `json:"color,omitempty"`
	Link      string `json:"link"`
	IsCurrent bool   `json:"is_current"`
	Total     int64  `json:"total"`
}

type LastVersion struct {
	Version          string `json:"version"`
	Description      string `json:"description"`
	TrialVersion     string `json:"trial_version"`
	TrialDescription string `json:"trial_description"`
	Trial            bool   `json:"trial"`
}

type ChartData struct {
	Date  string `json:"date"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

type PushLog struct {
	CreatedTime int64  `json:"created_time"`
	Spider      string `json:"spider"`
	Result      string `json:"result"`
}

type FindPasswordInfo struct {
	Way      string      `json:"way"`
	Token    string      `json:"token"`
	Host     string      `json:"host"`
	Verified bool        `json:"verified"`
	End      time.Time   `json:"-"`
	Timer    *time.Timer `json:"-"`
}

type TinyAttachment struct {
	FileName     string `json:"file_name"`
	FileLocation string `json:"file_location"`
}
