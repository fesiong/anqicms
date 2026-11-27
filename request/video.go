package request

type VideoTaskRetry struct {
	Id uint `json:"id"`
}

type VideoTaskHistorical struct {
	Force bool `json:"force"`
}
