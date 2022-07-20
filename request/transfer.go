package request

type TransferWebsite struct {
	Name     string `json:"name"`
	BaseUrl  string `json:"base_url"`
	Token    string `json:"token"`
	Provider string `json:"provider"`
}

type TransferRequest struct {
	Type   string `json:"type"`
	LastId int64  `json:"last_id"`
}
