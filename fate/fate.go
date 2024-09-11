package fate

import (
	"context"
	"errors"
	"fmt"
	"github.com/fesiong/yi"
	"github.com/goextension/log"
	"kandaoni.com/anqicms/fate/config"
	"kandaoni.com/anqicms/model"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godcong/chronos"
)

type FilterMode int

const (
	FilterModeNormal FilterMode = iota
	FilterModeHard
	FilterModeCustom
)

// HandleOutputFunc ...
type HandleOutputFunc func(name Name)

type Fate struct {
	config         *config.Config
	Born           chronos.Calendar
	Last           []string
	LastChar       []*model.Character
	First          []string
	FirstChar      []*model.Character
	Name           *Name //检查的时候用到
	Debug          bool
	BaZi           *BaZi
	Zodiac         *Zodiac
	Star           *Star
	Handle         HandleOutputFunc
	Position       string `json:"position"`        //方位：东南西北中
	SourceFrom     uint   `json:"source_from"`     //名字来源，论语、大学、中庸、诗经、周易、楚辞、尚书、道德经、唐诗、宋词、三字经、千字文、美好成语、不限 对应的id
	TabooCharacter string `json:"taboo_character"` //忌讳字
	TabooSide      string `json:"taboo_side"`      //忌讳偏旁部首
	OnlyKangxi     bool   `json:"only_kangxi"`     //指定只使用康熙字典
	NameType       string `json:"name_type"`       //姓名形式

	FilterMode   FilterMode `json:"filter_mode"`
	StrokeMax    int        `json:"stroke_max"` //指定最大笔画
	StrokeMin    int        `json:"stroke_min"` //指定最小笔画
	HardFilter   bool       `json:"hard_filter"`
	FixBazi      bool       `json:"fix_bazi"`      //八字修正
	SupplyFilter bool       `json:"supply_filter"` //过滤补八字
	ZodiacFilter bool       `json:"zodiac_filter"` //过滤生肖
	BaguaFilter  bool       `json:"bagua_filter"`  //过滤卦象
	Regular      bool       `json:"regular"`       //常用，排除生僻字
	LastName     []string   `json:"last_name"`
	Gender       string     `json:"gender"`
	AppointType  string     `json:"appoint_type"` //指定用字位置，first,second
	AppointName  string     `json:"appoint_name"` //指定用的字
}

// Options ...
type Options func(f *Fate)

// ConfigOption ...
func ConfigOption(cfg *config.Config) Options {
	return func(f *Fate) {
		f.config = cfg
	}
}

func (f *Fate) SetGender(gender string) {
	f.Gender = gender
}

func (f *Fate) PositionOption(position string) {
	f.Position = position
}

func (f *Fate) SourceOption(sourceFrom string) {
	source, _ := strconv.Atoi(sourceFrom)
	f.SourceFrom = uint(source)
}

func (f *Fate) TabooCharacterOption(char string) {
	f.TabooCharacter = char
}

func (f *Fate) TabooSideOption(char string) {
	f.TabooSide = char
}

func (f *Fate) OnlyKangxiOption(char bool) {
	f.OnlyKangxi = char
}

func (f *Fate) NameTypeOption(char string) {
	f.NameType = char
}

// Debug ...
func Debug() Options {
	return func(f *Fate) {
		f.Debug = true
	}
}

// NewFate 所有的入口,新建一个fate对象
func NewFate(lastName string, born time.Time, options ...Options) *Fate {
	f := &Fate{
		Last: strings.Split(lastName, ""),
		Born: chronos.New(born),
	}
	f.BaZi = NewBazi(f.Born)
	f.LastChar = make([]*model.Character, len(f.Last))
	if len(f.Last) > 2 {
		panic("last name was bigger than 2 characters")
	}

	for _, op := range options {
		op(f)
	}

	f.init()

	return f
}

func (f *Fate) getLastCharacter() error {
	size := len(f.Last)
	if size == 0 {
		return errors.New("last name was not inputted")
	} else if size > 2 {
		return fmt.Errorf("%d characters last name was not supported", size)
	} else {
		//ok
	}

	for i, c := range f.Last {
		character, ok := Character[c]
		if !ok {
			return errors.New("姓氏不存在")
		}
		f.LastChar[i] = character
	}
	return nil
}

var makeCh = make(chan bool, 10)

// MakeName ...
func (f *Fate) MakeName(ctx context.Context) (names []*Name, e error) {
	if len(makeCh) >= 10 {
		// 抛弃
		return nil, errors.New("QPS超负载")
	}
	makeCh <- true
	defer func() {
		<-makeCh
	}()
	e = f.getLastCharacter()
	if e != nil {
		return nil, Wrap(e, "get char failed")
	}
	name := make(chan *Name)
	go func() {
		e := f.getWugeName(name)
		if e != nil {
			log.Error(e)
		}
	}()
	var tmpChar []*model.Character
	//supplyFilter := false
	for n := range name {
		select {
		case <-ctx.Done():
			log.Info("end")
			return
		default:
		}

		tmpChar = n.FirstName
		tmpChar = append(tmpChar, n.LastName...)
		//filter bazi
		if f.config.SupplyFilter && !filterXiYong(f.XiYong().Shen(), tmpChar...) {
			//log.Infow("supply", "name", n.String())
			continue
		}
		//filter zodiac
		if f.config.ZodiacFilter && !filterZodiac(f.Born, n.FirstName...) {
			//log.Infow("zodiac", "name", n.String())
			continue
		}
		//filter bagua
		if f.config.BaguaFilter && !filterYao(n.BaGua(), "凶") {
			//log.Infow("bagua", "name", n.String())
			continue
		}
		//ben := n.BaGua().Get(yi.BenGua)
		//bian := n.BaGua().Get(yi.BianGua)
		//if f.debug {
		//	log.Infow("bazi", "born", f.born.LunarDate(), "time", f.born.Lunar().EightCharacter())
		//	log.Infow("xiyong", "wuxing", n.WuXing(), "god", f.XiYong().Shen(), "pinheng", f.XiYong())
		//	log.Infow("ben", "ming", ben.GuaMing, "chu", ben.ChuYaoJiXiong, "er", ben.ErYaoJiXiong, "san", ben.SanYaoJiXiong, "si", ben.SiYaoJiXiong, "wu", ben.WuYaoJiXiong, "liu", ben.ShangYaoJiXiong)
		//	log.Infow("bian", "ming", bian.GuaMing, "chu", bian.ChuYaoJiXiong, "er", bian.ErYaoJiXiong, "san", bian.SanYaoJiXiong, "si", bian.SiYaoJiXiong, "wu", bian.WuYaoJiXiong, "liu", bian.ShangYaoJiXiong)
		//}
		//
		//if f.debug {
		//	log.Infow(n.String(), "笔画", n.Strokes(), "拼音", n.PinYin(), "八字", f.born.Lunar().EightCharacter(), "喜用神", f.XiYong().Shen(), "本卦", ben.GuaMing, "变卦", bian.GuaMing)
		//}
		names = append(names, n)
	}
	//对name进行排序
	if f.Gender == "female" {
		sort.Sort(FemaleNames(names))
	} else {
		sort.Sort(MaleNames(names))
	}
	//fmt.Println(names)
	//最多保留200个
	total := len(names)
	if total > 200 {
		names = names[:200]
	}
	for i, _ := range names {
		names[i].baZi = f.BaZi
		names[i].born = &f.Born
	}
	return names, nil
}

func (f *Fate) CheckName(firstName string) {
	f.First = strings.Split(firstName, "")
	f.FirstChar = make([]*model.Character, len(f.First))
	f.getLastCharacter()

	var f2 *model.Character
	for i, c := range f.First {
		character, ok := Character[c]
		if !ok {
			return
		}
		if i == 1 {
			f2 = character
		}
		f.FirstChar[i] = character
	}

	name := createName(f, f.FirstChar[0], f2)
	name.born = &f.Born
	name.baZi = f.BaZi
	name.Zodiac()
	name.CiYu()
	name.BaGua()

	f.Name = name
}

// XiYong ...
func (f *Fate) XiYong() *XiYong {
	if f.BaZi == nil {
		f.BaZi = NewBazi(f.Born)
	}
	return f.BaZi.XiYong()
}

// XiYong ...
func (f *Fate) WuGe() *WuGe {
	var wuge *WuGe
	if len(f.LastChar) == 2 {
		wuge = CalcWuGe(f.LastChar[0].ScienceStroke, f.LastChar[1].ScienceStroke, 0, 0)
	} else {
		wuge = CalcWuGe(f.LastChar[0].ScienceStroke, 0, 0, 0)
	}

	return wuge
}

func (f *Fate) BaGua() *yi.Yi {
	lastSize := len(f.LastChar)
	shang := getStroke(f.LastChar[0])
	if lastSize > 1 {
		shang += getStroke(f.LastChar[1])
	}
	//未取名，没有
	xia := 0
	baGua := yi.NumberQiGua(xia, shang, shang+xia)

	return baGua
}

func (f *Fate) SanCai() *SanCai {
	l1 := getStroke(f.LastChar[0])
	l2 := 0
	lastSize := len(f.LastChar)
	if lastSize > 1 {
		l2 = getStroke(f.LastChar[1])
	}
	wuGe := &WuGe{
		tianGe: tianGe(l1, l2, 0, 0),
		renGe:  renGe(l1, l2, 0, 0),
		diGe:   diGe(l1, l2, 0, 0),
		waiGe:  waiGe(l1, l2, 0, 0),
		zongGe: zongGe(l1, l2, 0, 0),
	}

	sanCai1 := &SanCai{
		TianCai:        sanCaiAttr(wuGe.TianGe()),
		TianCaiYinYang: yinYangAttr(wuGe.TianGe()),
		RenCai:         sanCaiAttr(wuGe.RenGe()),
		RenCaiYinYang:  yinYangAttr(wuGe.RenGe()),
		DiCai:          sanCaiAttr(wuGe.DiGe()),
		DiCaiYingYang:  yinYangAttr(wuGe.DiGe()),
	}

	return sanCai1
}

func (f *Fate) init() {
	if f.config == nil {
		f.config = config.DefaultConfig()
	}

	//计算生肖
	f.Zodiac = GetZodiac(f.Born)
	//计算星座
	f.Star = GetStar(f.Born)
	//八字
	f.BaZi = NewBazi(f.Born)
}

// SetBornData 设定生日
func (f *Fate) SetBornData(t time.Time) {
	f.Born = chronos.New(t)
}

func (f *Fate) getWugeName(name chan<- *Name) (e error) {
	defer func() {
		close(name)
	}()
	lucky := make(chan *model.WuGeLucky)
	go func() {
		e = FilterWuGe(f.LastChar, f.AppointType, f.AppointName, lucky)
		if e != nil {
			return
		}
	}()

	var f1s []*model.Character
	var f2s []*model.Character
	nameCount := 0

	//如果指定了来源，则没有来源词的跳过
	var sourceNames = map[string][]*model.NameData{}
	if f.SourceFrom > 0 {
		var nameDatas []*model.NameData
		db.Where("source_id = ?", f.SourceFrom).Find(&nameDatas)
		for _, v := range nameDatas {
			key := fmt.Sprintf("%d-%d", v.FirstStroke1, v.FirstStroke2)
			sourceNames[key] = append(sourceNames[key], v)
		}
	}

	// 可能会重复，要过滤
	var exists = sync.Map{}
	var ch = make(chan int, 20)
	var wg = sync.WaitGroup{}
	for ll := range lucky {
		if f.Gender == "female" && filterSex(ll) {
			continue
		}
		if f.config.HardFilter && hardFilter(ll) {
			sc := NewSanCai(ll.TianGe, ll.RenGe, ll.DiGe)
			if !Check(sc, 5) {
				continue
			}
		}

		ch <- 1
		wg.Add(1)
		go func(l *model.WuGeLucky) {
			defer func() {
				<-ch
				wg.Done()
			}()
			f1s = Characters[l.FirstStroke1]

			if f.SourceFrom > 0 {
				tmpKey := fmt.Sprintf("%d-%d", l.FirstStroke1, l.FirstStroke2)
				tmpNames := sourceNames[tmpKey]
				if len(tmpNames) > 0 {
					for _, v := range tmpNames {
						if _, ok := exists.Load(v.FirstName); ok {
							continue
						}
						exists.Store(v.FirstName, struct{}{})
						//拆字
						firstName := strings.Split(v.FirstName, "")
						f1 := Character[firstName[0]]
						f2 := Character[firstName[1]]
						if f.AppointName != "" && f.AppointType == "first" {
							//指定第一个用字
							if f1.Ch != f.AppointName && f1.Radical != f.AppointName {
								continue
							}
						}
						//如果指定了康熙用字，则只显示康熙字典的字
						if f.OnlyKangxi && f1.KangXi == "" {
							continue
						}
						//如果指定了忌讳字，则遇到忌讳字跳过
						if f.TabooCharacter != "" && strings.Contains(f.TabooCharacter, f1.Ch) {
							continue
						}
						//如果指定了忌讳偏旁部首，则遇到偏旁字跳过
						if f.TabooSide != "" && strings.Contains(f.TabooSide, f1.Radical) {
							continue
						}
						// 如果指定了单字模式
						if f.NameType == "single" {
							if _, ok := exists.Load(f1.Ch); ok {
								continue
							}
							exists.Store(f1.Ch, struct{}{})
							nameCount++
							n := createName(f, f1, nil)
							name <- n
							continue
						}
						// 如果指定了叠字模式
						if f.NameType == "overlap" {
							if _, ok := exists.Load(f1.Ch); ok {
								continue
							}
							exists.Store(f1.Ch, struct{}{})
							//重字模式
							nameCount++
							n := createName(f, f1, f1)
							name <- n
							continue
						}
						if firstName[0] == firstName[1] {
							continue
						}
						n := createName(f, f1, f2)
						name <- n
					}
				}
			} else {
				for _, f1 := range f1s {
					if f.Gender == "female" {
						if f1.Female == 0 {
							continue
						}
					} else if f1.Male == 0 {
						continue
					}
					if f.AppointName != "" && f.AppointType == "first" {
						//指定第一个用字
						if f1.Ch != f.AppointName && f1.Radical != f.AppointName {
							continue
						}
					}
					//如果指定了康熙用字，则只显示康熙字典的字
					if f.OnlyKangxi && f1.KangXi == "" {
						continue
					}
					//如果指定了忌讳字，则遇到忌讳字跳过
					if f.TabooCharacter != "" && strings.Contains(f.TabooCharacter, f1.Ch) {
						continue
					}
					//如果指定了忌讳偏旁部首，则遇到偏旁字跳过
					if f.TabooSide != "" && strings.Contains(f.TabooSide, f1.Radical) {
						continue
					}
					// 如果指定了单字模式
					if f.NameType == "single" {
						if _, ok := exists.Load(f1.Ch); ok {
							continue
						}
						exists.Store(f1.Ch, struct{}{})
						nameCount++
						n := createName(f, f1, nil)
						name <- n
						continue
					}
					// 如果指定了叠字模式
					if f.NameType == "overlap" {
						if _, ok := exists.Load(f1.Ch); ok {
							continue
						}
						exists.Store(f1.Ch, struct{}{})
						//重字模式
						nameCount++
						n := createName(f, f1, f1)
						name <- n
						continue
					}
					// end
					count2 := 0
					var ok bool
					f2s, ok = Characters[l.FirstStroke2]
					if !ok {
						continue
					}
					for _, f2 := range f2s {
						// 普通情况，如果出现叠字，则跳过
						if f2.Ch == f1.Ch {
							continue
						}
						if f.Gender == "female" {
							if f2.Female == 0 {
								continue
							}
						} else if f2.Male == 0 {
							continue
						}
						// 过滤重复
						if _, ok := exists.Load(f1.Ch + f2.Ch); ok {
							continue
						}
						if _, ok := exists.Load(f2.Ch + f1.Ch); ok {
							continue
						}
						exists.Store(f1.Ch+f2.Ch, struct{}{})
						// 指定用字
						if f.AppointName != "" && f.AppointType != "first" {
							//指定第二个用字
							if f2.Ch != f.AppointName && f2.Radical != f.AppointName {
								continue
							}
						}
						//如果指定了康熙用字，则只显示康熙字典的字
						if f.OnlyKangxi && f2.KangXi == "" {
							continue
						}
						//如果指定了忌讳字，则遇到忌讳字跳过
						if f.TabooCharacter != "" && strings.Contains(f.TabooCharacter, f2.Ch) {
							continue
						}
						//如果指定了忌讳偏旁部首，则遇到偏旁字跳过
						if f.TabooSide != "" && strings.Contains(f.TabooSide, f2.Radical) {
							continue
						}
						if f.AppointName == "" {
							count2++
						}

						newTotal := f1.Total*f2.Total + int(math.Abs(float64(f1.Total-f2.Total))/2)
						if nameCount > 100 && (newTotal < 50000 || count2 > 5) {
							continue
						}

						nameCount++
						n := createName(f, f1, f2)
						//n.baZi = bazi
						name <- n
					}
				}
			}
		}(ll)

		if nameCount > 1000 {
			break
		}
	}
	wg.Wait()

	return
}

func filterSex(lucky *model.WuGeLucky) bool {
	return lucky.ZongSex == true
}

func isLucky(s string) bool {
	if strings.Compare(s, "吉") == 0 || strings.Compare(s, "半吉") == 0 {
		return true
	}
	return false
}

func hardFilter(lucky *model.WuGeLucky) bool {
	if !isLucky(yi.GetDaYan(lucky.DiGe).Lucky) ||
		!isLucky(yi.GetDaYan(lucky.RenGe).Lucky) ||
		!isLucky(yi.GetDaYan(lucky.WaiGe).Lucky) ||
		!isLucky(yi.GetDaYan(lucky.ZongGe).Lucky) {
		return true
	}
	return false
}
