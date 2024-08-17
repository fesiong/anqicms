package model

type HtmlPushLog struct {
	LocalFile   string `json:"local_file" gorm:"column:local_file;type:varchar(190);default '';primaryKey"` //文件路径
	RemoteFile  string `json:"remote_file" gorm:"column:remote_file;type:varchar(190);default ''"`          //文件路径
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index"`
	ModTime     int64  `json:"mod_time" gorm:"column:mod_time;type:int(11)"`
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1);default 0"`          //是否失败， 1成功，0失败
	ErrorMsg    string `json:"error_msg" gorm:"column:error_msg;type:varchar(255);default ''"` //错误信息
}
