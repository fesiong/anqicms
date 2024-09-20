package response

type WebLink struct {
	Name      string `json:"name"`
	Url       string `json:"url"`
	OriginUrl string `json:"origin_url"`
	Content   string `json:"content"`
}

