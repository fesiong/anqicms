package fate

import (
	"errors"
	"github.com/fesiong/yi"
	"kandaoni.com/anqicms/model"
	"math/rand"
	"time"
)

// WuGe ...
type WuGe struct {
	tianGe int
	renGe  int
	diGe   int
	waiGe  int
	zongGe int
}

// ZongGe ...
func (ge *WuGe) ZongGe() int {
	return ge.zongGe
}

func (ge *WuGe) WuGeScore() int {
	score := 100 - (ge.zongGe*ge.tianGe*ge.renGe*ge.diGe*ge.waiGe/10)%10
	return score
}

// WaiGe ...
func (ge *WuGe) WaiGe() int {
	return ge.waiGe
}

// DiGe ...
func (ge *WuGe) DiGe() int {
	return ge.diGe
}

// RenGe ...
func (ge *WuGe) RenGe() int {
	return ge.renGe
}

// TianGe ...
func (ge *WuGe) TianGe() int {
	return ge.tianGe
}

// CalcWuGe 计算五格
func CalcWuGe(l1, l2, f1, f2 int) *WuGe {
	return &WuGe{
		tianGe: tianGe(l1, l2, f1, f2),
		renGe:  renGe(l1, l2, f1, f2),
		diGe:   diGe(l1, l2, f1, f2),
		waiGe:  waiGe(l1, l2, f1, f2),
		zongGe: zongGe(l1, l2, f1, f2),
	}
}

// tianGe input the ScienceStrokes with last name
// 天格（复姓）姓的笔画相加
// 天格（单姓）姓的笔画上加一
func tianGe(l1, l2, _, _ int) int {
	if l2 == 0 {
		return l1 + 1
	}
	return l1 + l2
}

// renGe input the ScienceStrokes with name
// 人格（复姓）姓氏的第二字的笔画加名的第一字
// 人格（复姓单名）姓的第二字加名
// 人格（单姓单名）姓加名
//  人格（单姓复名）姓加名的第一字
func renGe(l1, l2, f1, _ int) int {
	//人格（复姓）姓氏的第二字的笔画加名的第一字
	//人格（复姓单名）姓的第二字加名
	if l2 != 0 {
		return l2 + f1
	}
	return l1 + f1
}

// diGe input the ScienceStrokes with name
// 地格（复姓复名，单姓复名）名字相加
// 地格（复姓单名，单姓单名）名字+1
func diGe(_, _, f1, f2 int) int {
	if f2 == 0 {
		return f1 + 1
	}
	return f1 + f2
}

// waiGe input the ScienceStrokes with name
// 外格（复姓单名）姓的第一字加笔画数一
// 外格（复姓复名）姓的第一字和名的最后一定相加的笔画数
// 外格（单姓复名）一加名的最后一个字
// 外格（单姓单名）一加一
func waiGe(l1, l2, _, f2 int) (n int) {
	//单姓单名
	if l2 == 0 && f2 == 0 {
		n = 1 + 1
	}
	//单姓复名
	if l2 == 0 && f2 != 0 {
		n = 1 + f2
	}
	//复姓单名
	if l2 != 0 && f2 == 0 {
		n = l1 + 1
	}
	//复姓复名
	if l2 != 0 && f2 != 0 {
		n = l1 + f2
	}
	return n
}

// zongGe input the ScienceStrokes with name
// 总格，姓加名的笔画总数  数理五行分类
func zongGe(l1, l2, f1, f2 int) int {
	//归1
	zg := (l1 + l2 + f1 + f2) - 1
	if zg < 0 {
		zg = zg + 81
	}
	return zg%81 + 1
}

func checkDaYan(idx int) bool {
	return isLucky(yi.GetDaYan(idx).Lucky)
}

// Check 格检查
func (ge *WuGe) Check() bool {
	//ignore:tianGe
	for _, v := range []int{ge.diGe, ge.renGe, ge.waiGe, ge.zongGe} {
		if !checkDaYan(v) {
			return false
		}
	}
	return true
}

func getStroke(character *model.Character) int {
	if character.ScienceStroke != 0 {
		return character.ScienceStroke
	} else if character.KangXiStroke != 0 {
		return character.KangXiStroke
	} else if character.Stroke != 0 {
		return character.Stroke
	} else if character.SimpleTotalStroke != 0 {
		return character.SimpleTotalStroke
	} else if character.TraditionalTotalStroke != 0 {
		return character.TraditionalTotalStroke
	}
	return 0
}

func FilterWuGe(last []*model.Character, appointType string, appointName string, wg chan<- *model.WuGeLucky) error {
	defer func() {
		close(wg)
	}()
	eng := db
	l1 := getStroke(last[0])
	l2 := 0
	if len(last) == 2 {
		l2 = getStroke(last[1])
	}
	s := eng.Model(&model.WuGeLucky{}).Where("last_stroke_1 =?", l1).
		Where("last_stroke_2 =?", l2).
		Where("zong_lucky = ?", 1)

	if appointType != "" && appointName != "" {
		appointChar, ok := Character[appointName]
		if !ok {
			return errors.New("汉字不存在")
		}
		f1 := getStroke(appointChar)
		if appointType == "first" {
			s = s.Where("first_stroke_1 = ？", f1)
		} else {
			s = s.Where("first_stroke_2 = ？", f1)
		}
	} else {
		s = s.Where("zong_da_yan = '吉' and ren_da_yan = '吉' and di_da_yan = '吉'")
	}

	var wgs []*model.WuGeLucky
	e := s.Find(&wgs).Error
	if e != nil {
		return e
	}
	// 随机排序
	shuffleWuGes(wgs)
	// end
	var existsF1 = map[int]int{}
	for _, v := range wgs {
		existsF1[v.FirstStroke1] = v.FirstStroke1
		if v.FirstStroke1 != v.FirstStroke2 && existsF1[v.FirstStroke2] > 0 {
			continue
		}
		wg <- v
	}

	return nil
}

func shuffleWuGes(slice []*model.WuGeLucky) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(slice) > 0 {
		n := len(slice)
		randIndex := r.Intn(n)
		slice[n-1], slice[randIndex] = slice[randIndex], slice[n-1]
		slice = slice[:n-1]
	}
}
