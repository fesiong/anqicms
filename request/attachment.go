package request

type Attachment struct {
	Id           uint   `json:"id"`            // 附件 ID
	FileName     string `json:"file_name"`     // 附件名称
	FileLocation string `json:"file_location"` // 附件存储位置
}

type AttachmentDeleteRequest struct {
	Id uint `json:"id"` // 附件 ID
}

type ChangeAttachmentCategory struct {
	CategoryId uint   `json:"category_id"` // 分类 ID
	Ids        []uint `json:"ids"`         // 附件 ID
}

type AttachmentAddRemoteUrl struct {
	CategoryId uint     `json:"category_id"` // 分类 ID
	Urls       []string `json:"urls"`        // 附件 URL
}
