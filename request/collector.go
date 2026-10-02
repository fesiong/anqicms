package request

type KeywordRequest struct {
	Id     uint   `json:"id"`               // 关键词 ID
	Title  string `json:"title"`            // 关键词名称
	Demand string `json:"demand,omitempty"` // AI 生成的额外要求(AI生成时提供)
}
