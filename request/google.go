package request

// PluginGoogleAuthRequest Google OAuth 回调请求。
type PluginGoogleAuthRequest struct {
	Code  string `json:"code"`  // 授权码，Google 回调带回
	State string `json:"state"` // 防 CSRF 的 state，必须与 auth/url 返回的一致
}

// PluginGoogleMerchantAccountRequest 选择要同步到哪个商户账号。
type PluginGoogleMerchantAccountRequest struct {
	MerchantId uint64 `json:"merchant_id"` // 商户 ID，从配置的 accounts 列表里选
}

// PluginGoogleMerchantSyncRequest 商品同步请求。
type PluginGoogleMerchantSyncRequest struct {
	Full bool `json:"full"` // 是否全量同步：true 全量重推，false 增量
	// Ids 指定要处理的商品 ID 列表。
	// ⚠️ sync 端点当前忽略此字段（固定走全量/增量）；只有 sync/delete 会用它。
	Ids []int64
}
