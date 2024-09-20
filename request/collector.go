package request

type KeywordRequest struct {
	Id     uint   `json:"id"`
	Title  string `json:"title"`
	Demand string `json:"demand,omitempty"` // AI 的额外要求
}
