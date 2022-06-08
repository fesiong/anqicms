package response

type AuthResponse struct {
	HashKey     string `json:"hash_key"`
	CreatedTime int64  `json:"created_time"`
	Code        string `json:"code"`
	Status      int    `json:"status"`
}

type Statistics struct {
	Total     int64 `json:"total"`
	Month     int64 `json:"month"`
	LastMonth int64 `json:"last_month"`
}

type SumAmount struct {
	Total int64 `json:"total"`
}

type LoginError struct {
	Times    int
	LastTime int64
}

type FilterGroup struct {
	Name      string       `json:"name"`
	FieldName string       `json:"field_name"`
	Items     []FilterItem `json:"items"`
}

type FilterItem struct {
	Label     string `json:"label"`
	Link      string `json:"link"`
	IsCurrent bool   `json:"is_current"`
	Total     int64  `json:"total"`
}
