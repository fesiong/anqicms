package response

type BaiduJson struct {
	Feed BaiduFeeJson `json:"feed"`
}

type BaiduFeeJson struct {
	Entry []BaiduEntryJson `json:"entry"`
}

type BaiduEntryJson struct {
	Title string `json:"title"`
	Url   string `json:"url"`
}
