package response

type DesignPackage struct {
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
	Docs  []DesignDoc `json:"docs"`
}

type DesignDoc struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Content string `json:"content"`
}
