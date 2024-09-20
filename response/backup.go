package response

type BackupInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	LastMod int64  `json:"last_mod"`
}
