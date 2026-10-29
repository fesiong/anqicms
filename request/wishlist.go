package request

type WishListRequest struct {
	Id          int64  `json:"id"`
	UserId      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	ArchiveId   int64  `json:"archive_id"`
	SkuId       int64  `json:"sku_id"`
	Title       string `json:"title"`
	SkuTitle    string `json:"sku_title"`
	Description string `json:"description"`
	Remark      string `json:"remark"`
	Thumb       string `json:"thumb"`
	Price       int64  `json:"price"`
	OriginPrice int64  `json:"origin_price"`
	TargetPrice int64  `json:"target_price"`
	Status      int    `json:"status"`
	CreatedTime int64  `json:"created_time"`
	UpdatedTime int64  `json:"updated_time"`
	Priority    int    `json:"priority"`
}
