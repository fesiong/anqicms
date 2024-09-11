package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

const JSONName = "config.json"

type FilterMode int

const (
	FilterModeNormal FilterMode = iota
	FilterModeHard
	FilterModeCustom
)

type OutputMode int

const (
	OutputModeLog OutputMode = iota
	OutputModeCSV
	OutputModelJSON
)

type FileOutput struct {
	OutputMode OutputMode `json:"output_mode"`
	Path       string     `json:"path"`
	Heads      []string   `json:"heads"`
}

type Config struct {
	RunInit        bool       `json:"run_init"`
	FilterMode     FilterMode `json:"filter_mode"`
	StrokeMax      int        `json:"stroke_max"` //指定最大笔画
	StrokeMin      int        `json:"stroke_min"` //指定最小笔画
	HardFilter     bool       `json:"hard_filter"`
	FixBazi        bool       `json:"fix_bazi"`      //八字修正
	SupplyFilter   bool       `json:"supply_filter"` //过滤补八字
	ZodiacFilter   bool       `json:"zodiac_filter"` //过滤生肖
	BaguaFilter    bool       `json:"bagua_filter"`  //过滤卦象
	Regular        bool       `json:"regular"`       //常用，排除生僻字
	FileOutput     FileOutput `json:"file_output"`
}

var DefaultJSONPath = ""
var DefaultHeads = []string{"姓名", "笔画", "拼音", "喜用神", "八字"}

func init() {
	if DefaultJSONPath == "" {
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		s, err := filepath.Abs(dir)
		if err != nil {
			panic(err)
		}
		DefaultJSONPath = s
	}
}

func LoadConfig() (c *Config) {
	c = &Config{}
	def := DefaultConfig()
	f := filepath.Join(DefaultJSONPath, JSONName)
	bys, e := ioutil.ReadFile(f)
	if e != nil {
		return def
	}
	e = json.Unmarshal(bys, &c)
	if e != nil {
		return def
	}
	return c
}

func OutputConfig(config *Config) error {
	bys, e := json.MarshalIndent(config, "", " ")
	if e != nil {
		return e
	}

	return ioutil.WriteFile(filepath.Join(DefaultJSONPath, JSONName), bys, 0755)
}

func DefaultConfig() *Config {
	return &Config{
		RunInit:      false,
		FilterMode:   0,
		StrokeMax:    20,    //限定最大笔画
		StrokeMin:    2,     //限定最小笔画
		HardFilter:   false, //偏硬
		FixBazi:      false, //八字修正
		SupplyFilter: true,  //过滤八字
		ZodiacFilter: true,  //过滤生肖
		BaguaFilter:  true,  //过滤卦象
		Regular:      true,  //常用字，避免生僻字

		FileOutput: FileOutput{
			Heads:      DefaultHeads,
			OutputMode: OutputModeLog,
			Path:       "name.txt",
		},
	}
}
