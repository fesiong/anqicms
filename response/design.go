package response

type DesignPackage struct {
	TemplateId   uint         `json:"template_id"`
	AuthId       uint         `json:"auth_id"`
	Name         string       `json:"name"`
	Package      string       `json:"package"`
	Version      string       `json:"version"`
	Description  string       `json:"description"`
	Author       string       `json:"author"`
	Homepage     string       `json:"homepage"`
	Created      string       `json:"created"`
	TemplateType int          `json:"template_type"`
	Status       int          `json:"status"`
	TplFiles     []DesignFile `json:"tpl_files"`
	StaticFiles  []DesignFile `json:"static_files"`
	PreviewData  bool         `json:"preview_data"`
}

type DesignFile struct {
	Path   string `json:"path"`
	Remark string `json:"remark"`

	Content string `json:"content,omitempty"`
	LastMod int64  `json:"last_mod,omitempty"`
	Size    int64  `json:"size"`
}

type DesignFileHistory struct {
	Hash    string `json:"hash"`
	Content string `json:"content,omitempty"`
	LastMod int64  `json:"last_mod,omitempty"`
	Size    int64  `json:"size"`
}
type DesignDocGroup struct {
	Title string      `json:"title"`
	Type  string      `json:"type"` // tag,filter
	Docs  []DesignDoc `json:"docs"`
}

type DocOption struct {
	Title string `json:"title"`
	Code  string `json:"code"`
}

type DesignDoc struct {
	Title   string      `json:"title"`
	Link    string      `json:"link"`
	Code    string      `json:"code"`
	Content string      `json:"content,omitempty"`
	Docs    []DesignDoc `json:"docs,omitempty"`
	Options []DocOption `json:"options,omitempty"`
}
