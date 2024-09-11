package fate

import (
	"fmt"
	"github.com/godcong/chronos"
	"math"
)

var hourIndex = map[int]string{
	0: "子", 1: "丑", 2: "寅", 3: "卯", 4: "辰", 5: "巳", 6: "午", 7: "未", 8: "申", 9: "酉", 10: "戌", 11: "亥",
}

var diIndex = map[string]int{
	"子": 0, "丑": 1, "寅": 2, "卯": 3, "辰": 4, "巳": 5, "午": 6, "未": 7, "申": 8, "酉": 9, "戌": 10, "亥": 11,
}

var tianIndex = map[string]int{
	"甲": 0, "乙": 1, "丙": 2, "丁": 3, "戊": 4, "己": 5, "庚": 6, "辛": 7, "壬": 8, "癸": 9,
}

var wuxing = map[string]string{
	"金": "金：金旺得火，方成器皿。<br/><br/>金能生水，水多金沉；强金得水，方挫其锋。<br/>金能克木，木多金缺；木弱逢金，必为砍折。<br/>金赖土生，土多金埋；土能生金，金多土变。",
	"木": "木：木旺得金，方成栋梁。<br/><br/>木能生火，火多木焚；强木得火，方化其顽。<br/>木能克土，土多木折；土弱逢木，必为倾陷。<br/>木赖水生，水多木漂；水能生木，木多水缩。",
	"水": "水：水旺得土，方成池沼。<br/><br/>水能生木，木多水缩；强水得木，方泄其势。<br/>水能克火，火多水干；火弱遇水，必不熄灭。<br/>水赖金生，金多水浊；金能生水，水多金沉。",
	"火": "火：火旺得水，方成相济。<br/><br/>火能生土，土多火晦；强火得土，方止其焰。<br/>火能克金，金多火熄；金弱遇火，必见销熔。<br/>火赖木生，木多火炽；木能生火，火多木焚。",
	"土": "土：土旺得水，方能疏通。<br/><br/>土能生金，金多土变；强土得金，方制其壅。<br/>土能克水，水多土流；水弱逢土，必为淤塞。<br/>土赖火生，火多土焦；火能生土，土多火晦。",
}

// 天干强度表
var tiangan = [][]int{
	{1200, 1200, 1000, 1000, 1000, 1000, 1000, 1000, 1200, 1200},
	{1060, 1060, 1000, 1000, 1100, 1100, 1140, 1140, 1100, 1100},
	{1140, 1140, 1200, 1200, 1060, 1060, 1000, 1000, 1000, 1000},
	{1200, 1200, 1200, 1200, 1000, 1000, 1000, 1000, 1000, 1000},
	{1100, 1100, 1060, 1060, 1100, 1100, 1100, 1100, 1040, 1040},
	{1000, 1000, 1140, 1140, 1140, 1140, 1060, 1060, 1060, 1060},
	{1000, 1000, 1200, 1200, 1200, 1200, 1000, 1000, 1000, 1000},
	{1040, 1040, 1100, 1100, 1160, 1160, 1100, 1100, 1000, 1000},
	{1060, 1060, 1000, 1000, 1000, 1000, 1140, 1140, 1200, 1200},
	{1000, 1000, 1000, 1000, 1000, 1000, 1200, 1200, 1200, 1200},
	{1000, 1000, 1040, 1040, 1140, 1140, 1160, 1160, 1060, 1060},
	{1200, 1200, 1000, 1000, 1000, 1000, 1000, 1000, 1140, 1140},
}

// 地支强度表
var dizhi = []map[string][]int{
	{
		"癸": {1200, 1100, 1000, 1000, 1040, 1060, 1000, 1000, 1200, 1200, 1060, 1140},
	}, {
		"癸": {360, 330, 300, 300, 312, 318, 300, 300, 360, 360, 318, 342},
		"辛": {200, 228, 200, 200, 230, 212, 200, 220, 228, 248, 232, 200},
		"己": {500, 550, 530, 500, 550, 570, 600, 580, 500, 500, 570, 500},
	}, {
		"丙": {300, 300, 360, 360, 318, 342, 360, 330, 300, 300, 342, 318},
		"甲": {840, 742, 798, 840, 770, 700, 700, 728, 742, 700, 700, 840},
	}, {
		"乙": {1200, 1060, 1140, 1200, 1100, 1000, 1000, 1040, 1060, 1000, 1000, 1200},
	}, {
		"乙": {360, 318, 342, 360, 330, 300, 300, 312, 318, 300, 300, 360},
		"癸": {240, 220, 200, 200, 208, 200, 200, 200, 240, 240, 212, 228},
		"戊": {500, 550, 530, 500, 550, 600, 600, 580, 500, 500, 570, 500},
	}, {
		"庚": {300, 342, 300, 300, 330, 300, 300, 330, 342, 360, 348, 300},
		"丙": {700, 700, 840, 840, 742, 840, 840, 798, 700, 700, 728, 742},
	}, {
		"丁": {1000, 1000, 1200, 1200, 1060, 1140, 1200, 1100, 1000, 1000, 1040, 1060},
	}, {
		"丁": {300, 300, 360, 360, 318, 342, 360, 330, 300, 300, 312, 318},
		"乙": {240, 212, 228, 240, 220, 200, 200, 208, 212, 200, 200, 240},
		"己": {500, 550, 530, 500, 550, 570, 600, 580, 500, 500, 570, 500},
	}, {
		"壬": {360, 330, 300, 300, 312, 318, 300, 300, 360, 360, 318, 342},
		"庚": {700, 798, 700, 700, 770, 742, 700, 770, 798, 840, 812, 700},
	}, {
		"辛": {1000, 1140, 1000, 1000, 1100, 1060, 1000, 1100, 1140, 1200, 1160, 1000},
	}, {
		"辛": {300, 342, 300, 300, 330, 318, 300, 330, 342, 360, 348, 300},
		"丁": {200, 200, 240, 240, 212, 228, 240, 220, 200, 200, 208, 212},
		"戊": {500, 550, 530, 500, 550, 570, 600, 580, 500, 500, 570, 500},
	}, {
		"甲": {360, 318, 342, 360, 330, 300, 300, 312, 318, 300, 300, 360},
		"壬": {840, 770, 700, 700, 728, 742, 700, 700, 840, 840, 724, 798},
	},
}

var wuXingTianGan = map[string]string{
	"甲": "木",
	"乙": "木",
	"丙": "火",
	"丁": "火",
	"戊": "土",
	"己": "土",
	"庚": "金",
	"辛": "金",
	"壬": "水",
	"癸": "水",
}

var wuXingDiZhi = map[string]string{
	"子": "水",
	"丑": "土",
	"寅": "木",
	"卯": "木",
	"辰": "土",
	"巳": "火",
	"午": "火",
	"未": "土",
	"申": "金",
	"酉": "金",
	"戌": "土",
	"亥": "水",
}

// WuXingTianGan 五行天干
func WuXingTianGan(s string) string {
	return wuXingTianGan[s]
}

// WuXingDiZhi 五行地支
func WuXingDiZhi(s string) string {
	return wuXingDiZhi[s]
}

// BaZi ...
type BaZi struct {
	baZi    []string
	wuXing  []string
	xiyong  *XiYong
	ShiChen string
	naYin   []string
}

// NewBazi 创建八字
func NewBazi(calendar chronos.Calendar) *BaZi {
	ec := calendar.Lunar().EightCharacter()
	return &BaZi{
		baZi:   ec,
		wuXing: baziToWuXing(ec),
		naYin:  nayinList(ec),
	}
}

// String ...
func (z *BaZi) String() string {
	str := "此命五行"
	wang := z.XiYong().GetWang()
	if wang != "" {
		str += fmt.Sprintf("【%s】旺，", wang)
	}
	que := z.XiYong().GetQue()
	if que != "" {
		str += "五行缺" + que + "，"
	}
	qiang := "弱"
	if z.XiYong().QiangRuo() {
		qiang = "强"
	}
	str += fmt.Sprintf("<span class=\"strong\">八字偏%s</span>，", qiang)
	str += fmt.Sprintf("八字喜【<span class=\"strong color-red\">%s</span>】，<span class=\"strong color-red\">%s</span> 就是此命的【喜神】，起名应以五行为 <span class=\"strong color-red\">%s</span> 的字来起名对宝宝成长，学业，事业更有利发展。", z.XiYongShen(), z.XiYongShen(), z.XiYongShen())
	second := z.XiYong().SecondShen()
	if second != "" {
		str += fmt.Sprintf("宝宝的次喜神为【<span class=\"strong color-primary\">%s</span>】，名字中包含 <span class=\"strong color-primary\">%s</span> 的字，同样可以改善宝宝的运势。", second, second)
	}

	return str
}

func (z *BaZi) GetBaZi(i int) string {
	return z.baZi[i*2] + z.baZi[i*2+1]
}

func (z *BaZi) GetWuXing(i int) string {
	return z.wuXing[i*2] + z.wuXing[i*2+1]
}

func (z *BaZi) GetNaYin(i int) string {
	return z.naYin[i]
}

// RiZhu 日主
func (z *BaZi) RiZhu() string {
	return z.baZi[4]
}

func (z *BaZi) RiZhuWuXing() string {
	return z.wuXing[4]
}

func (z *BaZi) calcXiYong() {
	z.xiyong = &XiYong{}
	//TODO:need fix
	z.point().calcSimilar().calcHeterogeneous().calcWuXing() //.yongShen().xiShen()
}

// XiYong 喜用神
func (z *BaZi) XiYong() *XiYong {
	if z.xiyong == nil {
		z.calcXiYong()
	}
	return z.xiyong
}

// XiYongShen 平衡用神
func (z *BaZi) XiYongShen() string {
	return z.XiYong().Shen()
}

//	func (z *BaZi) yongShen() *BaZi {
//		z.xiyong.YongShen = z.xiyong.Similar[0]
//		return z
//	}
//
//	func (z *BaZi) xiShen() *BaZi {
//		rt := sheng
//		if z.QiangRuo() {
//			rt = ke
//		}
//		for i := range rt {
//			if rt[i] == z.xiyong.YongShen {
//				if i == len(rt) {
//					i = -1
//				}
//				z.xiyong.XiShen = rt[i-1]
//				break
//			}
//		}
//		return z
//	}
func (z *BaZi) point() *BaZi {
	di := diIndex[z.baZi[3]]
	z.ShiChen = z.baZi[7] + "时"
	for idx, v := range z.baZi {
		if idx%2 == 0 {
			z.xiyong.AddFen(WuXingTianGan(v), tiangan[di][tianIndex[v]])
		} else {
			dz := dizhi[diIndex[v]]
			for k := range dz {
				z.xiyong.AddFen(WuXingTianGan(k), dz[k][di])
			}
		}
	}
	return z
}

func baziToWuXing(bazi []string) []string {
	var wx []string
	for idx, v := range bazi {
		if idx%2 == 0 {
			wx = append(wx, WuXingTianGan(v))
		} else {
			wx = append(wx, WuXingDiZhi(v))
		}
	}
	return wx
}

func nayinList(bazi []string) []string {
	nayin := []string{
		NaYinList[bazi[0]+bazi[1]],
		NaYinList[bazi[2]+bazi[3]],
		NaYinList[bazi[4]+bazi[5]],
		NaYinList[bazi[6]+bazi[7]],
	}

	return nayin
}

// 计算同类
func (z *BaZi) calcSimilar() *BaZi {
	for i := range sheng {
		if wuXingTianGan[z.RiZhu()] == sheng[i] {
			z.xiyong.Similar = append(z.xiyong.Similar, sheng[i])
			z.xiyong.SimilarPoint = z.xiyong.GetFen(sheng[i])
			if i == 0 {
				i = len(sheng) - 1
				z.xiyong.Similar = append(z.xiyong.Similar, sheng[i])
				z.xiyong.SimilarPoint += z.xiyong.GetFen(sheng[i])
			} else {
				z.xiyong.Similar = append(z.xiyong.Similar, sheng[i-1])
				z.xiyong.SimilarPoint += z.xiyong.GetFen(sheng[i-1])
			}
			break
		}
	}
	return z
}

// 计算异类
func (z *BaZi) calcHeterogeneous() *BaZi {
	for i := range sheng {
		for ti := range z.xiyong.Similar {
			if z.xiyong.Similar[ti] == sheng[i] {
				goto EndSimilar
			}
		}
		z.xiyong.Heterogeneous = append(z.xiyong.Heterogeneous, sheng[i])
		z.xiyong.HeterogeneousPoint += z.xiyong.GetFen(sheng[i])
	EndSimilar:
		continue

	}
	totalPoint := z.xiyong.SimilarPoint + z.xiyong.HeterogeneousPoint
	z.xiyong.SimilarPoint = z.xiyong.SimilarPoint * 10000 / totalPoint
	z.xiyong.HeterogeneousPoint = z.xiyong.HeterogeneousPoint * 10000 / totalPoint
	return z
}

func (z *BaZi) calcWuXing() *BaZi {
	totalFen := 0
	for _, v := range z.XiYong().WuXingFen {
		totalFen += v
	}
	var wuxing = map[string]int{}
	var wuxingNum = map[string]int{}
	maxWuxing := 0
	for _, v := range sheng {
		if z.xiyong.WuXingFen[v] == 0 {
			wuxing[v] = 0
			wuxingNum[v] = 0
		} else {
			wuxing[v] = int(math.Floor(float64(z.xiyong.WuXingFen[v])/float64(totalFen)*100 + 0.5))
			if wuxing[v] > maxWuxing {
				maxWuxing = wuxing[v]
			}
			wuxingNum[v] = 0
		}
	}
	for i, v := range wuxing {
		wuxing[i] = v * 100 / maxWuxing
	}
	z.xiyong.WuXing = wuxing

	for _, v := range z.wuXing {
		wuxingNum[v]++
	}

	z.xiyong.WuXingNum = wuxingNum
	return z
}

func (z *BaZi) GetBaZiScore() int {
	tong := math.Abs(float64(z.XiYong().SimilarPoint) / 100)
	yi := math.Abs(float64(z.XiYong().HeterogeneousPoint) / 100)
	zong := tong + yi

	return int(zong)
}

func (z *BaZi) GetWuXingString() string {
	wang := z.XiYong().GetWang()
	str := wuxing[wang]

	return str
}
