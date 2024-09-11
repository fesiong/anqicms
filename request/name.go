package request

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type NameCheckoutRequest struct {
	FullName   string `json:"full_name"`
	Gender     string `json:"gender"`
	KnowBorn   string `json:"know_born"`
	Calendar   string `json:"calendar"`
	Leap       string `json:"leap"`
	BornYear   string `json:"born_year"`
	BornMonth  string `json:"born_month"`
	BornDay    string `json:"born_day"`
	BornHour   string `json:"born_hour"`
	BornMinute string `json:"born_minute"`
}

type NameHoroscope struct {
	KnowBorn   string `json:"know_born"`
	Calendar   string `json:"calendar"`
	Leap       string `json:"leap"`
	BornYear   string `json:"born_year"`
	BornMonth  string `json:"born_month"`
	BornDay    string `json:"born_day"`
	BornHour   string `json:"born_hour"`
	BornMinute string `json:"born_minute"`
}

type NameRequest struct {
	LastName       string      `json:"last_name"`
	Gender         string      `json:"gender"`
	NameType       string      `json:"name_type"`
	KnowBorn       string      `json:"know_born"`
	Calendar       string      `json:"calendar"`
	Leap           string      `json:"leap"`
	BornYear       string      `json:"born_year"`
	BornMonth      string      `json:"born_month"`
	BornDay        string      `json:"born_day"`
	BornHour       string      `json:"born_hour"`
	BornMinute     string      `json:"born_minute"`
	Position       string      `json:"position"`
	PositionName   string      `json:"position_name"` // 具体的名字
	SourceFrom     interface{} `json:"source_from"`
	SourceName     string      `json:"source_name"` // 具体的名字
	OnlyKangxi     string      `json:"only_kangxi"`
	TabooCharacter string      `json:"taboo_character"`
	TabooSide      string      `json:"taboo_side"`
	AppointType    string      `json:"appoint_type"`
	AppointName    string      `json:"appoint_name"`
}

func (n NameRequest) Value() (driver.Value, error) {
	return json.Marshal(n)
}

func (n *NameRequest) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &n)
	return nil
}

func (n *NameRequest) GetGender() string {
	if n.Gender == "female" {
		return "女"
	}

	return "男"
}

func (n *NameRequest) GetSource() string {
	source := "美好词语"
	sourceFrom := fmt.Sprintf("%v", n.SourceFrom)
	switch sourceFrom {
	case "1":
		source = "论语"
		break
	case "2":
		source = "大学"
		break
	case "3":
		source = "中庸"
		break
	case "4":
		source = "诗经"
		break
	case "5":
		source = "周易"
		break
	case "6":
		source = "楚辞"
		break
	case "7":
		source = "尚书"
		break
	case "8":
		source = "道德经"
		break
	case "9":
		source = "唐诗"
		break
	case "10":
		source = "宋词"
		break
	case "11":
		source = "三字经"
		break
	case "12":
		source = "千字文"
		break
	case "13":
		source = "美好成语"
		break
	}
	n.SourceName = source

	return source
}

func (n *NameRequest) GetNameType() string {
	if n.NameType == "single" {
		return "单字名"
	}
	if n.NameType == "overlap" {
		return "叠字名"
	}
	return "双字名"
}

func (n *NameRequest) GetPosition() string {
	position := "不知道"
	switch n.Position {
	case "east":
		position = "东方"
		break
	case "northeast":
		position = "东北方"
		break
	case "southeast":
		position = "东南方"
		break
	case "south":
		position = "南方"
		break
	case "southwest":
		position = "西南方"
		break
	case "west":
		position = "西方"
		break
	case "northwest":
		position = "西北方"
		break
	case "north":
		position = "北方"
		break
	}
	n.PositionName = position

	return position
}
