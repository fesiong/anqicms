package request

type PluginGoogleAuthRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type PluginGoogleMerchantAccountRequest struct {
	MerchantId uint64 `json:"merchant_id"`
}

type PluginGoogleMerchantSyncRequest struct {
	Full bool `json:"full"` // 是否全量同步
	Ids  []int64
}
