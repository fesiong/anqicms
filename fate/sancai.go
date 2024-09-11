package fate

const sanCai = "水木木火火土土金金水"
const yinYang = "阴阳"

// SanCai ...
type SanCai struct {
	TianCai        string `bson:"tian_cai"`
	TianCaiYinYang string `bson:"tian_cai_yin_yang"`
	RenCai         string `bson:"ren_cai"`
	RenCaiYinYang  string `bson:"ren_cai_yin_yang"`
	DiCai          string `bson:"di_cai"`
	DiCaiYingYang  string `bson:"di_cai_ying_yang"`
	Fortune        string `bson:"fortune"` //吉凶
	Comment        string `bson:"comment"` //说明
}

//NewSanCai 新建一个三才对象
func NewSanCai(tian, ren, di int) *SanCai {
	return &SanCai{
		TianCai:        sanCaiAttr(tian),
		TianCaiYinYang: yinYangAttr(tian),
		RenCai:         sanCaiAttr(ren),
		RenCaiYinYang:  yinYangAttr(ren),
		DiCai:          sanCaiAttr(di),
		DiCaiYingYang:  yinYangAttr(di),
	}
}

//Check 检查三才属性
func Check(cai *SanCai, point int) bool {
	engine := db
	wx := FindWuXing(engine, cai.TianCai, cai.RenCai, cai.DiCai)
	if wx.Luck.Point() >= point {
		return true
	}
	return false
}

// GenerateThreeTalent 计算字符的三才属性
// 1-2木：1为阳木，2为阴木   3-4火：3为阳火，4为阴火   5-6土：5为阳土，6为阴土   7-8金：7为阳金，8为阴金   9-10水：9为阳水，10为阴水
func sanCaiAttr(i int) string {
	return string([]rune(sanCai)[i%10])
}

func yinYangAttr(i int) string {
	return string([]rune(yinYang)[i%2])
}
