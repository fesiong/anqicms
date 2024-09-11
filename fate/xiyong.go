package fate

import (
	"fmt"
	"kandaoni.com/anqicms/model"
	"strings"
)

// XiYong 喜用神
type XiYong struct {
	WuXingFen          map[string]int
	Similar            []string //同类
	SimilarPoint       int
	Heterogeneous      []string //异类
	HeterogeneousPoint int
	WuXingNum          map[string]int
	WuXing             map[string]int
}

var sheng = []string{"木", "火", "土", "金", "水"}
var ke = []string{"木", "土", "水", "火", "金"}

// AddFen 五行分
func (xy *XiYong) AddFen(s string, point int) {
	if xy.WuXingFen == nil {
		xy.WuXingFen = make(map[string]int)
	}

	if v, b := xy.WuXingFen[s]; b {
		xy.WuXingFen[s] = v + point
	} else {
		xy.WuXingFen[s] = point
	}
}

// GetFen 取得分
func (xy *XiYong) GetFen(s string) (point int) {
	if xy.WuXingFen == nil {
		return 0
	}
	if v, b := xy.WuXingFen[s]; b {
		return v
	}
	return 0
}

func (xy *XiYong) minFenWuXing(ss ...string) (wx string) {
	min := 9999
	for _, s := range ss {
		if xy.WuXingFen[s] < min {
			min = xy.WuXingFen[s]
			wx = s
		} else if xy.WuXingFen[s] == min {
			wx += s
		}
	}
	return
}

func (xy *XiYong) GetSimilar() string {
	return strings.Join(xy.Similar, "、")
}

func (xy *XiYong) GetHeterogeneous() string {
	return strings.Join(xy.Heterogeneous, "、")
}

func (xy *XiYong) GetSimilarPoint() string {
	return fmt.Sprintf("%.1f", float64(xy.SimilarPoint)/100)
}

func (xy *XiYong) GetHeterogeneousPoint() string {
	return fmt.Sprintf("%.1f", float64(xy.HeterogeneousPoint)/100)
}

func (xy *XiYong) GetZongPoint() string {
	tong := float64(xy.SimilarPoint) / 100
	yi := float64(xy.HeterogeneousPoint) / 100
	zong := tong - yi

	return fmt.Sprintf("%.1f", zong)
}

func (xy *XiYong) GetWuXing(str string) int {
	return xy.WuXing[str]
}

func (xy *XiYong) GetWuXingNum(str string) int {
	return xy.WuXingNum[str]
}

func (xy *XiYong) GetWang() string {
	var wang string
	max := 0
	for i, v := range xy.WuXingFen {
		if v > max {
			wang = i
			max = v
		}
	}

	return wang
}

func (xy *XiYong) GetQue() string {
	var que []string
	for _, v := range sheng {
		if xy.WuXingFen[v] == 0 {
			que = append(que, v)
		}
	}

	return strings.Join(que, "、")
}

// Shen 喜用神
func (xy *XiYong) Shen() string {
	if !xy.QiangRuo() {
		return xy.minFenWuXing(xy.Similar...)
	}
	return xy.minFenWuXing(xy.Heterogeneous...)
}

func (xy *XiYong) SecondShen() string {
	shen := xy.Shen()
	second := ""
	if !xy.QiangRuo() {
		if len(xy.Similar) > 1 {
			for _, xx := range xy.Similar {
				if xx != shen {
					second = xx
					break
				}
			}
		}
	} else {
		if len(xy.Heterogeneous) > 1 {
			for _, xx := range xy.Heterogeneous {
				if xx != shen {
					second = xx
					break
				}
			}
		}
	}

	return second
}

// QiangRuo 八字偏强（true)弱（false）
func (xy *XiYong) QiangRuo() bool {
	return xy.SimilarPoint > xy.HeterogeneousPoint
}

func (xy *XiYong) GetQiangRuo() string {
	str := "弱"
	if xy.QiangRuo() {
		str = "强"
	}

	return str
}

func filterXiYong(yong string, cs ...*model.Character) (b bool) {
	for _, c := range cs {
		if c.WuXing == yong {
			return true
		}
	}
	return false
}
