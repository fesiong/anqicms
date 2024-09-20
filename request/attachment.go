package request

type Attachment struct {
	Id       uint   `json:"id"`
	FileName string `json:"file_name"`
}

type AttachmentCategory struct {
	Id    uint   `json:"id"`
	Title string `json:"title"`
}

type ChangeAttachmentCategory struct {
	CategoryId uint   `json:"category_id"`
	Ids        []uint `json:"ids"`
}
