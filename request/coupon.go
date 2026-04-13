package request

type CouponRequest struct {
	Id           int64  `json:"id"`
	Money        int64  `json:"money"`
	StartTime    int64  `json:"start_time"`
	EndTime      int64  `json:"end_time"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Expire       int64  `json:"expire"`
	Status       int    `json:"status"`
	Type         int    `json:"type"`
	MaxMoney     int64  `json:"max_money"`
	MoneyType    int    `json:"money_type"`
	NeedIntegral int64  `json:"need_integral"`
	NeedMoney    int64  `json:"need_money"`
	RangeType    int    `json:"range_type"`
}

type CouponCodeRequest struct {
	Id        int64   `json:"id"`
	UserId    int64   `json:"user_id"`
	CouponId  int64   `json:"coupon_id"`
	Code      string  `json:"code"`
	StartTime int64   `json:"start_time"`
	EndTime   int64   `json:"end_time"`
	Status    int     `json:"status"`
	Money     int64   `json:"money"`
	Ids       []int64 `json:"ids"`
}

type CouponArchiveRequest struct {
	CouponId   int64   `json:"coupon_id"`
	ArchiveId  int64   `json:"archive_id"`
	ArchiveIds []int64 `json:"archive_ids"`
}

type MakeCouponCodeRequest struct {
	CouponId int64 `json:"coupon_id"`
	Count    int   `json:"count"`
	UserId   int64 `json:"user_id"`
}
