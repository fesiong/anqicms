package fate

import (
	"fmt"
	"github.com/fesiong/yi"
	"github.com/godcong/chronos"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"math"
	"strconv"
	"strings"
)

const (
	//好听内涵
	ScoreInner = iota
	//五行八字
	ScoreBaZi
	//生肖打分
	ScoreZodiac
	//星座打分
	ScoreStar
	//五格理数
	ScoreWuGe
	//周易卦像
	ScoreYi
)

type NameDayan struct {
	TianGe yi.DaYan `json:"tian_ge"`
	DiGe   yi.DaYan `json:"di_ge"`
	RenGe  yi.DaYan `json:"ren_ge"`
	WaiGe  yi.DaYan `json:"wai_ge"`
	ZongGe yi.DaYan `json:"zong_ge"`
}

// Name 姓名
type Name struct {
	FirstName   []*model.Character //名姓
	LastName    []*model.Character
	ciYu        []*model.NameSourceData
	SourceId    uint
	born        *chronos.Calendar
	baZi        *BaZi
	baGua       *yi.Yi //周易八卦
	zodiac      *Zodiac
	wuGe        *WuGe
	TotalCount  int
	MaleCount   int
	FemaleCount int
	zodiacPoint int
	Scores      map[int]int
	lastName    string
	firstName   string
	gender      string
}

type TotalNames []*Name
type MaleNames []*Name
type FemaleNames []*Name

func (a TotalNames) Len() int           { return len(a) }
func (a TotalNames) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TotalNames) Less(i, j int) bool { return a[i].TotalCount > a[j].TotalCount }

func (a MaleNames) Len() int           { return len(a) }
func (a MaleNames) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a MaleNames) Less(i, j int) bool { return a[i].MaleCount > a[j].MaleCount }

func (a FemaleNames) Len() int           { return len(a) }
func (a FemaleNames) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a FemaleNames) Less(i, j int) bool { return a[i].FemaleCount > a[j].FemaleCount }

// String ...
func (n Name) String() string {
	var s string
	for _, l := range n.LastName {
		s += l.Ch
	}
	for _, f := range n.FirstName {
		s += f.Ch
	}
	return s
}

// Strokes ...
func (n Name) Strokes() string {
	var s []string
	for _, l := range n.LastName {
		s = append(s, strconv.Itoa(l.ScienceStroke))
	}

	for _, f := range n.FirstName {
		s = append(s, strconv.Itoa(f.ScienceStroke))
	}
	return strings.Join(s, ",")
}

// PinYin ...
func (n Name) PinYin() string {
	var s string
	for _, l := range n.LastName {
		s += "[" + strings.Join(l.PinYin, ",") + "]"
	}

	for _, f := range n.FirstName {
		s += "[" + strings.Join(f.PinYin, ",") + "]"
	}
	return s
}

// WuXing ...
func (n Name) WuXing() string {
	var s string
	for _, l := range n.LastName {
		s += l.WuXing
	}
	for _, f := range n.FirstName {
		s += f.WuXing
	}
	return s
}

// XiYongShen ...
func (n Name) XiYongShen() string {
	return n.baZi.XiYongShen()
}

func (n Name) WuGe() *WuGe {
	if n.wuGe == nil {
		l1 := 0
		l2 := 0
		f1 := 0
		f2 := 0
		l1 = n.LastName[0].ScienceStroke
		f1 = n.FirstName[0].ScienceStroke
		if len(n.LastName) > 1 {
			l2 = n.LastName[1].ScienceStroke
		}
		if len(n.FirstName) > 1 {
			l2 = n.FirstName[1].ScienceStroke
		}

		n.wuGe = CalcWuGe(l1, l2, f1, f2)
	}
	return n.wuGe
}

// DaYan ...
func (n Name) DaYan() NameDayan {
	wuge := n.WuGe()

	nameDayan := NameDayan{
		TianGe: yi.GetDaYan(wuge.DiGe()),
		DiGe:   yi.GetDaYan(wuge.DiGe()),
		RenGe:  yi.GetDaYan(wuge.RenGe()),
		WaiGe:  yi.GetDaYan(wuge.WaiGe()),
		ZongGe: yi.GetDaYan(wuge.ZongGe()),
	}

	return nameDayan
}

func createName(impl *Fate, f1 *model.Character, f2 *model.Character) *Name {
	lastSize := len(impl.LastChar)
	last := make([]*model.Character, lastSize)
	copy(last, impl.LastChar)
	lastName := strings.Join(impl.Last, "")
	firstName := f1.Ch

	totalCount := f1.Total
	maleCount := f1.Male
	femaleCount := f1.Female
	first := []*model.Character{f1}
	if f2 != nil {
		firstName += f2.Ch
		first = append(first, f2)
		totalCount = f1.Total*f2.Total + int(math.Abs(float64(f1.Total-f2.Total))/2)
		maleCount = f1.Male*f2.Male + int(math.Abs(float64(f1.Male-f2.Male))/2)
		femaleCount = f1.Female*f2.Female + int(math.Abs(float64(f1.Female-f2.Female))/2)
	}

	return &Name{
		FirstName:   first,
		LastName:    last,
		SourceId:    impl.SourceFrom,
		firstName:   firstName,
		lastName:    lastName,
		gender:      impl.Gender,
		born:        &impl.Born,
		TotalCount:  totalCount,
		MaleCount:   maleCount,
		FemaleCount: femaleCount,
	}
}

func (n *Name) CiYu() []*model.NameSourceData {
	if n.ciYu == nil {
		n.ciYu = []*model.NameSourceData{}
		num := 0
		for i := range NameSources {
			if n.SourceId > 0 && n.SourceId != i {
				continue
			}
			for _, item2 := range NameSources[i] {
				if index := strings.Index(item2.Title, n.FirstName[0].Ch); index != -1 {
					if len(n.FirstName) > 1 {
						if index2 := strings.Index(item2.Title[index:], n.FirstName[1].Ch); index2 != -1 {
							n.ciYu = append(n.ciYu, item2)
							num++
							if num >= 5 {
								break
							}
						}
					} else {
						n.ciYu = append(n.ciYu, item2)
						num++
						if num >= 5 {
							break
						}
					}
				}
			}
			if num >= 5 {
				break
			}
		}
	}
	return n.ciYu
}

func CountNameSource(ch1 string, ch2 string) int64 {
	var total int64
	builder := db.Model(&model.NameSourceData{}).Where("FIND_IN_SET(?, `search_key`)", ch1)
	if ch2 != "" {
		builder = builder.Where("FIND_IN_SET(?, `search_key`)", ch2)
	}
	builder.Count(&total)

	return total
}

func (n *Name) Zodiac() *Zodiac {
	if n.zodiac == nil {
		n.zodiac = GetZodiac(*n.born)
	}
	return n.zodiac
}

// BaGua ...
func (n *Name) BaGua() *yi.Yi {
	if n.baGua == nil {
		lastSize := len(n.LastName)
		shang := getStroke(n.LastName[0])
		if lastSize > 1 {
			shang += getStroke(n.LastName[1])
		}
		firstSize := len(n.FirstName)
		xia := getStroke(n.FirstName[0])
		if firstSize > 1 {
			xia += getStroke(n.FirstName[1])
		}
		n.baGua = yi.NumberQiGua(xia, shang, shang+xia)
	}

	return n.baGua
}

func (n *Name) Gender() string {
	if n.gender == "female" {
		return "女"
	}
	return "男"
}

func (n *Name) FullName() string {
	return n.lastName + n.firstName
}

func (n *Name) GetLastName() string {
	return n.lastName
}

func (n *Name) GetFirstName() string {
	return n.firstName
}

func (n *Name) GetUrl() string {
	//将它转换成一串字符串
	born := *n.born
	bornTime := int(born.Solar().Time().Unix())

	md5Hash := library.Md5(fmt.Sprintf("%s-%s-%s-%d", n.lastName, n.firstName, n.gender, bornTime))
	md5Hash = md5Hash[8:24]

	detail := &model.NameDetail{
		Hash:      md5Hash,
		LastName:  n.lastName,
		FirstName: n.firstName,
		Gender:    n.gender,
		Born:      bornTime,
	}
	go checkName(detail)

	uri := fmt.Sprintf("/detail/%s", md5Hash)

	return uri
}

func checkName(detail *model.NameDetail) {
	db.Clauses(clause.OnConflict{
		DoNothing: true,
	}).Where("`hash` = ?", detail.Hash).Create(detail)
}

// BaZi ...
func (n Name) BaZi() *BaZi {
	return n.baZi
}

func (n *Name) TotalScore() int {
	if len(n.Scores) == 0 {
		n.Scores = n.CalcScores()
	}
	return (n.Scores[ScoreInner]*20 + n.Scores[ScoreBaZi]*20 + n.Scores[ScoreZodiac]*10 + n.Scores[ScoreStar]*10 + n.Scores[ScoreWuGe]*20 + n.Scores[ScoreYi]*20) / 100
}

func (n *Name) GetScore(num int) int {
	if len(n.Scores) == 0 {
		n.Scores = n.CalcScores()
	}
	return n.Scores[num]
}

func (n *Name) GetCiYuScore() int {
	score := 90
	other := n.FirstName[0].Total
	if len(n.FirstName) > 1 {
		other += n.FirstName[1].Total
	}
	other = other / 100
	if other > 10 {
		other = 10
	}
	score += other

	//ciLen := len(n.CiYu())
	//score += ciLen
	//if ciLen > 9 {
	//	score -= 1
	//}
	return score
}

func (n *Name) CalcScores() map[int]int {
	var scores = map[int]int{}
	scores[ScoreInner] = n.GetCiYuScore()
	scores[ScoreBaZi] = n.baZi.GetBaZiScore()
	scores[ScoreZodiac] = n.Zodiac().GetZodiacScore(n.FirstName...)
	scores[ScoreStar] = GetStarScore(*n.born)
	scores[ScoreWuGe] = n.WuGe().WuGeScore()
	scores[ScoreYi] = GetYiScore(n.baGua)
	n.Scores = scores
	return scores
}
