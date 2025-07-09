package library

import (
	"fmt"
	"log"
	"testing"
)

func TestParseDescription(t *testing.T) {
	str := "广西玉林市博白县凤山中学一学生因点外卖，被校务人员按倒在地训斥教育，登上网络热搜。3月21日上午，博白县教育局回应记者时表示，有工作人员在调查处理此事，并将适时公布调查情况。网络视频显示，一名男生被一名黑衣男子按倒在地，该黑衣男子大声说道：“点多少次了啊，还这样搞……”随后，记者与广西玉林市博白县凤山中学取得联系，相关负责人称，视频中的男生是因为点外卖被批评，学校对安全管理很严格，外面食品不能带进学校的，另外已针对该校务人员的工作方法进行了批评教育。据央视网报道，3月20日上午，牙冠竞价挂网于在四川成都举行并产生入围结果。口腔牙齿种植的费用大致包括种植体、牙冠和医疗服务费用三部分。其中，种植体集中带量采购已于今年1月开展，中选产品价格平均降至900多元，平均降幅55%；医疗服务方面，此前国家医保局发文要求，三级公立医院单颗常规种植牙医疗服务价格调控目标为4500元，多地已根据要求出台或落实了相关政策；叠加此次牙冠竞价挂网，预计种植一颗牙的整体费用有望降低50%左右。"

	desc := ParseDescription(str)

	log.Println(desc)
}

func TestParseContentTitles(t *testing.T) {
	str := `<h2>H2 title</h2>
<h3>H3 title</h3>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h2>H2 title</h2>
<h3>H3 title</h3>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h3>H3 title</h3>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h3>H3 title</h3>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h2>H2 title</h2>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h2>H2 title</h2>
<h3>H3 title</h3>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
<h4>H4 title</h4>
`

	titles, content := ParseContentTitles(str, "list")

	for i, title := range titles {
		fmt.Println("Level1", i, title.Prefix, title.Tag, title.Level, title.Title)
		for j, child := range title.Children {
			fmt.Println("Level2", i, j, child.Prefix, child.Tag, child.Level, child.Title)
			for k, child2 := range child.Children {
				fmt.Println("Level3", i, j, k, child2.Prefix, child2.Tag, child2.Level, child2.Title)
				for l, child3 := range child2.Children {
					fmt.Println("Level4", i, j, k, l, child3.Prefix, child3.Tag, child3.Level, child3.Title)
				}
			}
		}
	}
	log.Println(content)
}
