package fate

import (
	"github.com/fesiong/yi"
)

// QiGua 起卦
func QiGua(xia, shang int) *yi.Yi {
	return yi.NumberQiGua(shang, xia)
}

func GetYiScore(baGua *yi.Yi) int {
	score := 0
	score += baGua.Get(0).ShangShu * baGua.Get(0).XiaShu
	score += baGua.Get(1).ShangShu * baGua.Get(1).XiaShu
	score += baGua.Get(2).ShangShu * baGua.Get(2).XiaShu
	score += baGua.Get(3).ShangShu * baGua.Get(3).XiaShu
	score += baGua.Get(4).ShangShu * baGua.Get(4).XiaShu

	score = 100 - score%10

	return score
}
