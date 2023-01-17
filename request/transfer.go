package request

type TransferWebsite struct {
	Name     string `json:"name"`
	BaseUrl  string `json:"base_url"`
	Token    string `json:"token"`
	Provider string `json:"provider"`
}
