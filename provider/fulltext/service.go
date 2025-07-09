package fulltext

const (
	CategoryDivider = 900000000000000000
	TagDivider      = 800000000000000000

	ArchiveType  = "archive"
	CategoryType = "category"
	TagType      = "tag"
)

type TinyArchive struct {
	Type        string `json:"type"` // archive, category, tag
	Id          int64  `json:"-" gorm:"column:id"`
	ModuleId    uint   `json:"module_id"`
	Title       string `json:"title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

func (a TinyArchive) GetId() int64 {
	var id = a.Id
	if a.Type == CategoryType {
		id = CategoryDivider + a.Id
	} else if a.Type == TagType {
		id = TagDivider + a.Id
	}

	return id
}

func GetId(id int64) (int64, string) {
	if id >= CategoryDivider {
		return id - CategoryDivider, CategoryType
	} else if id >= TagDivider {
		return id - TagDivider, TagType
	}

	return id, ArchiveType
}

type Service interface {
	Index(body interface{}) error
	Create(doc TinyArchive) error
	Update(doc TinyArchive) error
	Delete(doc TinyArchive) error
	Bulk(docs []TinyArchive) error
	Search(keyword string, moduleId uint, page int, pageSize int) ([]TinyArchive, int64, error)
	Close()
	Flush()
}
