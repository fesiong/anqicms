package fate

import "github.com/fesiong/yi"

func getYao(xiang *yi.GuaXiang, yao int) *yi.GuaYao {
	return xiang.GuaYaos[yao]
}

func filterYao(y *yi.Yi, fs ...string) bool {
	yao := getYao(y.Get(yi.BianGua), y.BianYao())
	for _, s := range fs {
		if yao.JiXiong == s {
			return false
		}
	}
	return true
}
