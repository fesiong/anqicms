package fate

import (
	"github.com/godcong/chronos"
	"kandaoni.com/anqicms/model"
	"strings"
)

// ZodiacPig ...
const (
	ZodiacMice   = "鼠"
	ZodiacCattle = "牛"
	ZodiacTiger  = "虎"
	ZodiacRabbit = "兔"
	ZodiacDragon = "龙"
	ZodiacSnake  = "蛇"
	ZodiacHorse  = "马"
	ZodiacSheep  = "羊"
	ZodiacMonkey = "猴"
	ZodiacChick  = "鸡"
	ZodiacDog    = "狗"
	ZodiacPig    = "猪"
)

// Zodiac 生肖
type Zodiac struct {
	Id        string
	Name      string
	Xi        string //喜
	XiRadical string
	Ji        string //忌
	JiRadical string
}

var zodiacList = map[string]Zodiac{
	ZodiacMice: {
		Id:        "mice",
		Name:      ZodiacMice,
		Xi:        "上于口大女子巾丑中丹云井互元勻升壬太戶方月水牛王世丘主令充冬加北卉古右司台巨市布本民永玄玉生用田由甲申禾立丞亥兆先再匡同各名后回好如宇守安曲有牟米羽自舟衣西江池串亨兌利助吾呂君告吹吟妍妤孚宏局希彤杉甫男秀見言谷豆貝車辰里町沅沛汪沐沂玖事享京兒函叔和固奇姍孟宗定官宜宙尚居岸帛店府承服朋枋果松知秉竺采金長阜雨青汯育怡注泳河波法油治玥冠勃勁厚品客宥屋帝度建彥思扁柱柔架柚皇盈省相眉科貞軍韋飛首泰柘肴芳芸洲洪流津洞活洛珊玲珍唐哥員唇娟家宮容宸展峰師桐桀畔益真祐神祝素純紜訓財貢軒高紘胞胤若苗悟悅振海涓浚浴浩琉珮凰動區商問寂專將崇常康彬彩啟旋毫笛笙統紹細紳翎袈訪責貫野鹿浤婕娸翊草茵茗捷涼淳添清淇淵涵深淦理現勝博單喬婷媚富寓幀惠期棟棉甥登發程童粟紫絲絡舒詠賀貴鈞閎雅茜淀淥琇淼詒莎莉莆揮援港湛湘湖渙湄琪琳琴琛琦琨勤嗣園圓嫁廉敬新業楚榆毓督祺祿萬經絹聖裕詩詰農鈴雷靖頓鼎粲豊郡菩萍菁華溶源滄溪瑞瑜嘉圖實彰旗榕榮榛槐睿碩禎福管精綽綾綠綱綺綢綿綸維肇舞裴誦誌語誥豪賓輔銅銘銓閣領魁鳳滕瑋嫺陞葦漢滿瑤嫻嬋嬌寬廣慶慧瑩稼穀稻締緯緻緣誼諒諄論賞賦鋒震頡槿緗鋐霈靚鄉蓉蒲蒼澄潔潭潤瑾器學寰樺橙樹機積穎糖縈翰臻興親諺謀諭豫錄錦錡霖靜龍蓁璇隆蔗濂濃澤嬪嶸檐爵禧穗糠績聰襄謙闊隸鴻璠檉謚濟濠濤濬濰環璦璨禮穠繡謹豐鎔顓蕎鎧蕾璿璽禱穩繹贊鏗韻鯨麒麗薆瀅藏懷瀚瓊羅勸寶朧繼覺警鐘馨瀧瀠藩藝藤瓏櫻譽躍鐸露顧鶴蘋蘇瓖懿權讀灃孋穰鑌龢蘭巖欐麟纕瓚讓靈靄鑫觀鑲鑰黌靉酈灣讚灤鑼豔鸞",
		XiRadical: "宀米豆鱼艹金玉人木月田钅亻",
		Ji:        `丁二人士仁仃仇仍今介化午夭孔巴日火以付仕代仙仟卯央平旦未汀伙伊伍休仲件任仰仳份企光全印合地在圭妃寺早旨旬旭朴次竹羊臣行仵价伂位住伫伴佛何估佐佑伺伸似但作伯低伶余佈免坊址坍均坎壮孝宋巫志攸杜赤辛佟抑亚佯依侍佳使供例来佰佩仑佾侑兔味命坪坡坦妹岱幸旺易昌昆昂明昀昏昊昇东杵炎祀卧佼佶侄坵旻炅耵肖拒沫亭亮信侯侠保促俟俊俗俐係俞勉南哇型垠垣垢城姜姿宣封庠律徉春昭映昧是星昱柯柏柳炫为炳炬炯红美订酊香俋昶炷肯洋倍俸倩倖俩值倚倨俱倡候修倪俾伦冤冻卿埂埔埃姬娩宰差徐恙时晋晏晃晁书桓柴氧烊烈留羔耿袁袂酒钉马珋倢埕埒晟邦那迎胡苎苣茉英茆挽珠停假偃偌做伟健偶侦倏冕曼唱域坚堆埠基堵执婚张得从悠教晚晤曹勗欲焉烯瓷祥羚聊袋许顶偈偅偩偮娅欷欸羕邱迪悻傢傅备杰喜尧堪场堤堰报堡复普晰晴晶景智曾棚款焰然善翔贷距辜集冯傌媄焯羢郎郁脩莘莫荷提扬佣傲传仅僇吗塑塘涂塚塔填塌块坞妈微意暗晖暖暄会楠杨歇照煜焕义羡群详路钾铆驰驯渼羟郝连造逢菟署僧像侨境垫墅寿彻榴歌熙熊监绽台赫輓驳墉杩都逸进慢漾漫亿仪僵价侬俭刘墟增坠墩嬉德徵暮样槽楼欧羯蝴卖辉锌养驻驷驶驾驹鲁儆僾儌墡禡羬陈运道达潜骂儒傧壁垦坛壅曆晓昙炽燕熹笃糕羲谘醒骆嶪烨羱阳邹远优偿储壕壑壎曙营灿镁鍚骋骏鲜鲑邬蓨澲燨膳丛垒曜柠欤缮缯题骑镏骐郑邓选薑嚥垄坜曝羶羹辞鹏鄯薘繨邺迈壤曦耀议骞腾骝牺驱蓦欢鑑鞑骄骅惊驿验坝骤驞骥骊`,
		JiRadical: "",
	},
	ZodiacCattle: {
		Id:        "cattle",
		Name:      ZodiacCattle,
		Xi:        "乙乃了二几下凡也于千士子寸小川己巳弓丑丹之云井元內勻壬孔引斗水世丘冬北卉巧平弘永玄生田甲禾穴立丞亥兆再地妃好守州曲臣虫西池坊均壯妙妤孝宏序廷步甫私秀角豆邑酉乳亞享其函叔和固姍孟季定宜宙尚居延承果松牧秉竺舍金隹雨非芋芍法泓治泛勃勁厚垂屋建弈扁癸盈相科秒秋竿虹要重風飛食泰芊迅巡芳芝芽芹芸芷洲津凌原圃家庫庭料栗特畔耘耕航記訓起軒酒配芫邦那迎近范若苗英苔苑挺浦海浮浚浩浥乾基專康張強笠笙翌麥苹苡浤婕娸述迪草茵荏茲捷淳添清涵淦凱博喜壹富巽弼甥登發程稅筍筑粟舒貴鈕鈞閎雅雄順黍茜淀淥畯迺莆湘渙湄圓廉彙楓毓猷萬稜稚稠艇鈿雋雷電楙豊逍通逗連逐透逢菩萍菁華菱菽菲菊萄源榮槐睿筵翠臺豪輔酵銅銘銨閩鳴鳳逑菀菫菘菉溱溎滎箐嫺逵週逸進萱葵葦葉漳演漢漣滬嫻寬廣彈毅稼穀稻範諄趟趣輪鋪鋒黎逯萩滽鋐郵運遊道蓉蒼潼澄潭潤潘勳奮學導樺橙穎築臻興融豫錠錄錐雕霍霏頤蓁潗澐廩隆遠遙蔗蓮蔬蔭澤篷聲臨鍵鍾鴻鄘鄞適遨濟濠濛濯濬濡獲蟬豐鎰馥鵑蕓薌鎧鄰鄭鄧選薪蕾穫醮鏗鵬薐瀅鏞還邁藍薰薹寶醴馨薷邇藩藝籐蠟鶯鶴邊藺蘋蘇蘊懿權藶蘭麟艷鷹黌",
		XiRadical: "水艹豆米金玉宀人木氵钅亻",
		Ji:        `丁人刀匕大干仁什仃仇今介化午天夭心戈支斤日月比片牙王爿丙主以付仕代令仙刊加功仟占古召叱央它弗必旦未末玉矛矢示汀伙伊伍休仲件任仰仳份企光全列刑合宇年式戎早旨旭有朱此竹米羊行衣仵价伂忙托位住佇伴佛何估佐佑伺伸佔似但作伯低伶余佈別判君呀吟坎巫希庇弟彤形忌志忍攸杉系赤辛佟礽快玖些佯依侍佳使供例來佰佩侖佾侑刻券刷到制協卓味命奈妹宗岡岱岳幸弦忠忽念或戕旺易昌昆昂明昀昏昕昊昇服朋枝杵欣版狀直知社祀祁糾臥初采青忻玕佼佶侄佌旻炘炅耵罕育怔怖怪怕怡性沫玨玟玫玥表亭信侯保促俟俊俗俐係前剋則勇南奎奐姜宣庠弭彥徉思急怎怨春昭映昧是星昱柔架柏矜祉祈祇穿紂紅紀紉紇約紆美羿衫訂酊玡俋姝昶柰祅紈肯芯恰恢恆恃恬指洋洧珊玲珍珀玳倍俸倩倖倆值倚倨俱倡候修倪俾倫剛夏娥宰峻峰差恙恣恐恕恩時晉晏晃晁書朔朗栩格氧烊矩祕祐祠祟祖神祝秘紡紗紋素索純紐級紜納紙紛羔翁耿袂衽託躬釘馬倢倧扆晟祜紘紓衿衾邪胖胥胡胞胤苧茅茉苓悄悟悍悔悅悖班琉珮珠停假偃偌做偉健偶偵倏副務勘動曼唱唬國婚將崇崙彬彩彫御悉悠教旋晚晤晨曹勗望桿梧欲烯祥祭絃統紮紹紼細紳組終羚翎習聆彪被袒袖袍袋許責頂珖玼偈偅偩偮崟欷欸紵紾羕袗胭脈能悽情悻惜惟悸掛採捺淺琅球理現傢傅備傑剴創勞勝場堤報彭悲惠斯普晰晴晶景智曾期朝棕椅棧棹棉款牌番短結絨絕紫絮絲絡給絢善翔翕裁裂視診費貸辜集須馮琁珺傌媄斌牋甯矞絪絜羢軫陀郁脩莘莫莊慨惶愉提揚琪琳琥琴琯琦琨傭傲傳僅僇勤勢嗎嗣媽愚意慈感想愛愁愈愍戡暗暉暖暄會業楠楊歇照牒祺祿禁經絹綏義羨群裟裙補裘裝裕詳試路鉀雉零馳馴愃渼琮絻羥郡郝慎慌慄慍愧準瑚瑟瑞瑙瑛瑜署僧像僑劃寥廖彰慇態截暢歌碧禎福禍粽綻綰綜綽綾綠緊綴網綱綺綢綿綵綸維緒緇綬臧裴裸製褚赫趕駁瑋榪綪緁緆緋綖綦裱魠慷慢慣慚漾漫瑤瑣瑪瑰億儀僵價儂儉劇劉劍增嬉審影德慧慰慾暮樣槽槳歐獎瑩締練緯緻緘緬編緣緞緩緲緹羯翩褐複褓褊質輝鋅養駐駟駛駕駒魯齒摎漻儆僾儌墡禡緗羬褌褋褙陳達憐憎憤潛澎璋璃璀罵儒儐劑憩戰曆曉曇熾燕禦篤糕縑縈縣縝縉羲褪褲褫醒駱璇璉嶪縕羱錼陽膠蔣憶憾璞優償儲勵嶸彌應懂懇懋戲戴曙牆矯禧禪績繆縷總縱縴縵翼褸鎂鍚騁駿鮮蓨澲燨蟉襁膳蕙環璨叢斷曜檸歟璧禮織繕繞繚繡繒顏題騎鄢蕥騏際臆薑璿嚥壞曝牘璽疆矇禱繫繹繩繪繳羶羹襠襟識辭鄯薘繯繨鄴藏懷瓊勸懸曦朧繽繼耀議騫騰齡繾瓏儷櫻犧纏續驅驀懼瓔彎歡禳襯韃驕驊鷚戀曬纓纖驚驛驗襴鷸隴驟鑫驞纘灣驥纜驤驪`,
		JiRadical: "",
	},
	ZodiacTiger: {
		Id:        "tiger",
		Name:      ZodiacTiger,
		Xi:        "上大子山巾公午升壬太心月木水犬王丘主令出北卯巨市布必本永玄玉穴立汀亥匡年戎成有朱竹羊羽肉衣求江汐坊壯妙孝宋宏岑希彤志李材杉角走肌沁沛汪沐沂玖乳卓奇妹孟定岡岳帖忠念承服朋東林杰松武采長雨青忻汯育怡性注泳泌法玨玥冠勉勁南城奎威宥帝帥彥柱柄柳癸皇眉美衫軍音首泰怜姵肱肴肯恆恬洋洞洧玲珍剛展峻峰師恕恭恩息朕根桂栩栽桀桃烈純馬恂珂峮紘悅浪海浮浚琉珮珠珪崧常康彩敏梓梅盛絃統翌翎浤婕婧翊脈情惇捷淳添清淋涵深淨淦猛琅球理琍勝尊崴惠朝棠棟森棋棚然絢翔象雅雄集淥珺琈媏嵋淼絜愉湧湘渤滋渙猶琺琳琴琛琦媽意慈想愛業楚楠榆楣歲毓煌猷絹群聖裘裕靖馳馴鼎湜琰琭媵絺慎溪瑞瑜嫣寧彰旗榮睿端綻綽綠綱綺綵維緒翡豪滕溎瑂瑍槊滎箐箖綪緁綦慣滿漪瑤瑪慰樟樂毅締緯緹駐緗霈緣憬潔璋學憲篤縈翰錦霖霏靜潚潗璆叡橚燁憶撼澳璟璞嶸懋總駿鴻黛檍醡膳濟濠濯濬濰璦璨檸繕繡璱騏璿璽繹鵬瀅濼瓀懷瀚瓊繼騫騰瀠攘籐續巖驛",
		XiRadical: "山玉金木示水月犭马氵钅",
		Ji:        `一二人几三凡口士小川已弓丰之仁仇仍今介化夭少尤屯引日乏以付仕仗代仙兄冉加包仟可古右召司另只史台句央尼平弘弗旦民生田由甲申皮示禾氾伙伊伍休仲件任仰仳份企光全再危吉同各向名合后因回妃如存尖帆早旨旭朵次米臣虫仵价伂圮汎亨位住伴佛何估佐佑伺伸佃似但作伯低伶余佈克吾吳呈呂告含吟尾巫廷弟彷役攸杏甫男甸私秀谷豆貝辰邑佟冏町礽艾亞享依侍佳使供例來佰佩侖佾侏侑卷味咖咕呻和周命固坤奈委季宗宛尚岱延弦往旺易昌昆昂明昀昏昊昇知社祀祁秉臥虎門佼佶侄侗旻炅拒招河泓治泡泛亭亮信侯俠俏保促侶俟俊俗俐係俞咨咸品哈姿宣客巷建弭律思春昭映昧是星昱柯柏炬炯畏界祉祈祇禹科秒秋虹貞頁風香芊芃俋昶柰祅迅巡芳芝芙芹花芬芸芷倍俸倩倆值倚倨俱倡候修倪俾倫倉唐哥哲員娟姬孫容宸庭弱徐時晉晏晃晁書桐畔留益砷祕祐祠祟祖神祝租秦秩秘素虔袁訊財貢起躬酒閃高芮珅倢倧晟祜邢邦那迎近胡范苣若茂茉苒苗英苔苑苓苯捐假偃偌做偉健偶偵倏凰區曼商啞唱唯啤售國堅堂婉婚崇庸張強得從徠悠啟晚晤晨曹勗梁欲烯瓷略畦畢異祥祭移紹細紳處彪蛇蛉袖袍袋責頂麥苻偈偅偩偮埏婭欷欸邵邱述迦迪荊荐草茵荏茲茹茶茗荀茱荃捺涎傢傅備傑凱喧喜單喚喬喉場堤堠壹媛富巽幅弼彭復循敦普晰晴晶景智曾替棕棻款番稍程稅稀粟結給善虛蛟視費賀貴貿貸超距酥閔開閑間閎項順須黃黍茜傌郎郁送迷迺脩莎莞莘莫莒莊莓莉荷荻莆提揚渭傭傲傳僅僇募嗣園微暗暉暖暄會楊歇照當畸祺祿禁萬稜稚羨號蜀蛻蜂資賈農鉀鉅鈿預頌鼓莩琬媴郝通連速造逢逖途菩萍菁華菱著萊萌菽菲菊搭溶猿瑛署僧像僑僎嘉團圖壽夢幕徹暢歌熔熙監禎福禍種稱粽精綜臺蜜蜻誘賓閨閩頗鳴鳳逑菖菇溒榬蜒郭都逵週逸進蒂落萱葵葉葛董慢漠漫億儀僵價儂儉增嬌德徵慕暮槽歐皺稿稼稽稷蝴蝶誕豎賦賢賣賜輝頡魯黎萲儆僾儌稹陳鄉運遊道達違過遁蓉蓄蒙蒞蒲蓓蒼撰潛澎潘儒儔儐器戰曆曉暹曄曇樺橋盧禦積穆糕糖臻螢融諮謂諷賴醒頤龍蓁蒨螈錼陽鄒遠遜遣遙遞蔽蔚蓮蔓蔣蔡蓬擋優償儲彌懂曙檔禧禪穗臨襄轅鍚闆鄔蓨蔘鄞適遷蕩蕃蕉蕭曜歟禮穠糧繒蟬豐鎔闕顏題鵑蕎鎱際鄧選遲薪薑薛薇薊薦嚥曝疇疆禱穫穩譁贊關靡類顛薆譔還邁邀藏薩藍藉薰薹嚴曦耀麵鏵饌邈藩藝藤儷顧驀邊藻蘋蘇蘊彎歡疊禳鑑鄺灃驊蘭曬顯蘩囑蘴蘿蠻邏灣`,
		JiRadical: "",
	},
	ZodiacRabbit: {
		Id:        "rabbit",
		Name:      ZodiacRabbit,
		Xi:        "口士女寸小巾才中丹尹云互允公匹壬少屯戶方月木水牙四世丘加北卯可古右司只台句尼巨市平弗朮本未母永玄甘用田由甲申目石禾穴亦亥共再吉同各向名因回地好如字守曲朱竹米羊而臣自至舟衣求汝江串亨兌克冶利呈呂告坊妍妤孝宋宏局希彤李杏材村杉甫男秀見谷豆里妘町沙沛沐沂乳事亞享京其典函叔味和周固委妹孟季定官宜宙宛尚居岳幸於朋杭枋東果林松直知秉空竺舍采雨汯芋芍拓泳河沽法油泗治罔表亭亮勉勁匍咨咸品哈垂城奕姜姿威宦客屋弈彥扁柿柔柵柯柄柚柳泉界癸盈省相眉科紀約美耐韋泰芎咭姞姝姵畇肪芳芝芹芬芥芸拾洋洲洪津洞洛洧倉凍凌卿原哨唐哥哲圃娜娟姬娌孫宰家宮容展旅桂桔桀格株畝畜留益租秦窈紡純級紜舫軒高芮洙洺桉紘邦那若苗英苔苑浦海涓浮浚浴浩涌區商國基夠寄尉常康彬彩教啟敏旋旌梁梓梵梅梨略畢異盛祥笠笛笙絃統紹細紳耜野苹挹浤婕婧娸草茵茴茲茶茗捷涼淳添清淇淋淵涵深淨淦凱喜單喬壹壺婷媚富彭敦棠棟森棣棋棉畫登程童策筆答筍紫絮絲給絳善肅舒超開閑閒閎黍淀淥喨婼淼畯絜雱雰莎莘莒莊荷莆握湘湖渦滋渙湄園圓嫁廉彙敬業楚極楓榆楣毓當祺萬稠經絹綏義羨群聖肆裔裘裕雷渼嫄椲碇粲菩萍菁華菊溶源嘉圖壽夢嫦彰榕榮構槐榭睿碩福種管精綻綽綠綺綢綿綵綸緒臺豪輔魁菀菉溱嫥槊榞箐箖綦都萱葦葫葉葡董演漾漢滿嘻嫻嬋嬌寬廣樣樟樁樞樓樂樑毅磊磐稿稼穀稷稻範篆篇練緯緻緘緣緞豎黎萩葒滽嘽槿緗霈陵陳陸陰陶膏蓉蒿蓄蒙蒲蓓蒼澄潔潘勳器學寰樺橙樹橡橋機穌篙築糕糖縈縉羲臻興親豫輻輯霖靜蒡蓁蒨叡圜廩縕隊隋隆蔗蔬蔭據擇澤澳孺擎爵磺臨螺襄豁霜霞蔘檡霝龠鄘蕃蕭濟濠濛濤濬濡叢嚮檬歸禮穡簧繕繡繙豐魏蕎薌濧闓薪薄蕾櫥疇穫繹薀瀅藏薩藍瀟瀝嚴繼馨薷瀠繻藩藝藤瀾欄籐藟藜韡藻蘇灃龢蘭欒靈黌豔",
		XiRadical: "月艹山田人禾木宀金白玉豆钅亻",
		Ji:        `人大山仁仇今介化天太夭尤心日氏王主以付仕代令仙仟必旦民玉伙伊伍休仲件任仰仳份企光全存宇安早旨旬旭有羽西仵价伂忙位住伴佛何估佐佑伺伸似但作伯低伶余佈君巫忌志忍攸貝辰酉佟快玖依侍佳使供例來佰佩侖佾侑命岱忠忽念旺易昌昆昂明昀昏昊昇祀臥金長青忻佼佶侄旻炅育怔怖怪怕怡性玫玥信侯俠保促俟俊俗俐係冠宣帝思急怎怨春昭映昧是星昱柏羿要貞酋酊飛俋肯恰恢恆恃恬珊玲珍珀倍俸倩倆值倚倀倨俱倡候修倪俾倫宸恣恐恕恭恩息時晉晏晃晁書朔栩烏秘素翅翁財酒配倢晟栖翃胖胥胡胞胤苓悄悟悍悔悅悖振班琉珮珠乾假偃偌做偉健偶偵倏凰曼唱唬婚崔崗帳張悉悠晚晤晨曹勗望欲烯翌翎習聆彪袖袋責酗釧雀鳥鹿悒珝偈偅偩偮欷欸翊胭脈能悽情悵惜惟悸琅球理現傢傅備傑勝場堤就悲惠普晰晴晶景暑智曾期朝款翔翕覃貽費賀貴買貸酥鈞雄集順珺傌媜崷惢甯郁脩莫慨惶愉提揚琪琳琴琦琨傭傲傳僅僇愚意慈感想愛愁愈愍暗暉暖暄會楊楨歇照稚資賈賄賂農酬酪鉀鈴零愃煪詡慎慌慄慍愧瑚瑟瑞瑙瑛瑜署僧像僑寥實廖慇態暢碧禎翠翡翟裴賓銀鉻領鳴鳳愫瑋嵺嶍慷慢慣慚漲漫瑤瑣瑪瑰億儀僵價儂儉增慶慧慕慰慾暮槽歐瑩賠賣賜質輝醇醋銳鋒震駐魯憀摎漉漻儆僾儌槢樛潁熠熤翦憐憎憤潛璋璃璀儒儐憲憩曆曉曇歙穎翰翱翮賴醒錯錢鋼錄錦頭頤鴦鴒鴛龍遒璇璉醐醑醍鋹鴗陽膠憶憾濃璞優償儲嬰應懂懇懋曙績繆翳翼褶賺賽購鍵鍊鍚鴻鴿鄔蔍蓨嚁簏韔膳蕙環璨曜歟璧穠繒翹翻蹟醫鎮雙顏題鵑鵠燿豂轆轇鄭遺臆薑璿嚥壞龐曝璽矇醮鏡願鵡鵲鵬麒麗麓騛鵰懷瀚瓊嚨寶懸曦朧耀鐘騰齡藋瀧顟飂鶖鶧瓏儷躍鐵顧鶯鶴麝趯鶼蘋懼瓔懿歡籠襲鑑龔譾鷚戀曬纓纖顯鷥麟鷸鷲隴鷹鷺囑鸐酈鑽鑾鸕鸚鸞鸝`,
		JiRadical: "",
	},
	ZodiacDragon: {
		Id:        "dragon",
		Name:      ZodiacDragon,
		Xi:        "上大子巾丹今午升壬天太孔日曰月水王主北巨市永玉申立汀丞光兆好字存宇旭有羽衣求汝江池汐亨君孝希彤李言酉沛汪沂乳函坤孟定承旺易昌昆明昀昕昊昇朋杳杵杲采長雨青育注泳泗泊玨玥冠勃勁南厚奕帝彥映是星昱柱柄柏泉皇計音飛泰姵昶股肴肯津洛洧洵玲珍展師時晉晏晃書朔桂桀真素純紜袁酒馬珂珅衿海浙浮浩琉珮珠珪凰將常彩晞統紳翌翎習婕婧笭翊捷淳添清淋涵深淦球勝媛晰晴晶景智期朝棟森棉氯皓紫絮絢翔註詠雅雄雲須淀淥珺琈淼筌絜揚港湘湖湯琳琴琦琨暖暄暘會楠盟睛祿絹聖詩詣詮雷馳馴湜郡瑚瑜彰暢榮睿綽綠綺綿綵維翡舞裳誥趙鳳瑄瑔禔箐箖綦漳漢滿瑤瑪樟樁瑩篁締緻緣緹誼諄賜輝醇震皞霈靚潔潮璋勳學曉曇縉翰諦諺諭錦霖靜澐諟陽澤璟曙爵績總謙謄霞駿鴻濬濡濰環璨曜繡鵑璿璽繹譚贊鵬麗瀅瓊繽耀釋飄騰瀠籐靂靈讚驤",
		XiRadical: "水金玉白赤月鱼酉人氵钅亻",
		Ji:        `二人刀匕口士小山川工巳干弓丰仁什仇介元允化匹夭少尤屯巴引心戈戶支斤木比片牙犬爿以付仕代令仙充兄冉冊刊加功仟卉占卯可古右召司另史叱台句央它尼巧平弘弗必戊田由甲矛矢禾穴伙伊伍休仲件任仰仳份企先全再列刑匡匠印吉同各向名合后因回如宅安尖屹州式戎戌戍成旨曲朴次此米羊聿臣艮仵价伂旮忙托位住伴佛何估佐佑伺伸佃佔似但作伯低伶余佈克免別判利刪助吾吳呈呂告吠呀含吟壯完宋宏局尾岑岌巫序庇廷弟忌志忍攸束杏杜牢甫男甸私秀究良谷豆邑佟冏艾快抑狄狂些亞依侍佳使供例來佰佩侖佾侑兔刻券刷到制協卓味咖咕和周命奇委姍官宜宙宛尚屈居屆岷岡岸岩岱岳庚店府底庖延弦忠忽念或戕房所昂服東枝林松欣武版狀直知秉空臥舍虎門忻玕佼佶侄佌宓炘罕肖怔怖怪怕怡性拒招河泓治狗狐罔亭亮信侯保促俟俊俗俐係前剋則勇勉咨咸品哈城姜姿威宣室客宥屏屋峙峒巷度建弭思急怎怨扁春昭柔柵柯柳炯畏界畎相矜科秒秋穿突竿美虹芊芃狘玡俋峋峸巡肩芳芝芙芽花芬芸恰恢恆恃恬指洲洞狩珊倍俸倩倆值倚倨俱候修倪俾倫冤凍剛卿唐哥哲哭員圃埔娟姬娩宰家宴宮容宸峽峻峨峰島席庫庭座弱恣恐恕恩息朗桓柴桐畔留益矩租秦秩秘窈虔託躬高狨珋倢宬彧晟毧邢邪邦那迎近胖胥胡胞胤范茅苣若茂茉苗英苔苑苓苯茆悄悟悍悔悅悖捐挽浦假偃偌做偉健偶偵偏倏冕副務勘動匿區商唱問唯啤售國域堅堂寇寅寄寂宿密專屠崇崆崑崔崙崧崗巢康庸庶庵張強彗悉悠扈啟晚曹梁桿欲瓷略畢異盛移窕符紹細羚聊處彪袋雪麥苻玼偈偅偩偮婭欷欸馗邵邱迪胭脈能荊草茵荏茲茹茶荀悽情惜惟悸捨淺猜猛傢傅備傑剴創勞博喧喜單喚喬壹孱富寓尋就嵐崴巽幅廊廂弼彭惑悲悶惠扉敦斯棚棻款然牌犀短稍程稅稀窘窗粟結絨給善舒費貸越超距酥閔開閎黃黍邰茙淢傌斌棫牋甯矞羢陀郎郁迷脯脩莎莞莘莫莒莊莓莉荷荻莆慨惶愉減渭猶猴傭傲傳僅僇募勤勢匯嗣園塞嵩廉廈愚意慈感想愛愁愈愍戡新楚歇照牒猷當畸萬稜稚窟義羨群號蜀裝裕試誠資農鉀鉅鈿鉚雉鼓郕莩愃猭傮稢豊鉞通連造逢菩萍菁華菱萊萌菲菊菟慎慌慄慍愧搭滅準獅猿瑛僧像僑兢劃匱嘉嘈團圖壽夢寞寧寥實察屢幕廓廖慇態截榕榴歌熔熙獄監福種稱精聞肇臧臺蜜誘豪賓趕輔輓銅閨閩閣鳴菇獂瑊瑋屣廕戫魠郭都逵逸進蒂落萱葵葉葛董慷慢慣慚演漠漕滬億儀僵價儂儉劇劉劍嬌寬審層履廚廟廣廠慶慧慕慰慾暮槽樓槳歐獎稿稼稽稷窯窮箴篇糊編翩蝴蝶豎賢賣質閱養黎齒葴萲儆僾儌稹羬舖陳鄉運道達遍蓉蓄蒙蒞蒲蓓蒼憐憎憤澎潺潤潘潯儒儐劑器寰憲憩戰樺橋熾燃盧積穆窺糕糖羲臻螢融諮謂鋸頤默蓁廩諴駥遠膠蔽蔚蓮蔓蔣蔡蓬憶憾擋獨優償儲勵嶺嶽嶸彌應懂懇懋戲戴檔牆矯禪穗糟臨艱襄賽闊闈闆隇蓨蔘鍼遭遷膳蕊蕙蕩蕃蕉蕭濤獲斷歟禮竄竅糧織蟬豐鎔闖闕蕎蕕蕥鎦鄭鄧選臆膺薪薑薇獵薦壞寵龐牘獸疇疆矇穫穩譁識關靡類騙薆還邁藏藍藉薰懷勸嚴寶懸朧獻竇醴闡麵齡鏵騮藩藝藤瀾儷屬巍欄闢驀藻蘋蘇蘊懼巒彎懿權歡疊鑑鄺灃驊蘭巖戀竊纖驚蘩鷸鷢隴囑鷹蘴蘿廳灣`,
		JiRadical: "",
	},
	ZodiacSnake: {
		Id:        "snake",
		Name:      ZodiacSnake,
		Xi:        "乙丁乃二上凡也于口士夕大寸小己已巾弓丑中丹之尹井互午升巴心方月木毛火牙牛丙且包可古右司台句尼巨弘必本未末札玄玉甘生用田由甲目穴再吉同向后因回妃如宇守宅安寺帆年曲有朱朵竹羊羽肉臣自色西兌助吾呈呂君告吟宋宏床彤志束杏材村杞杉甫男角言辰邑酉里町忱事亞京味奇宗定官宜宙宛尚居幸忠念朋杭枋東果林杰松杵炎牧的長阜隹青非忻宓炘育怡性玥亭冠勇勉匍南卻咸品哈奎姜姚宦客宥巷帝帥建彥思柱柵柯柄柚柳炫為炳炬炯界盈紅紀美虹計貞軍面韋飛咭姵畇迅肯芳恆恬恪珊玲珍唐圃娜娟宰宮容宸展師庫庭恕恩息桂桔栩栗桐桀桃烈真純紐記訊訓起邕配馬高珂紓邦那迎若苗悟悅振挺珮區唯國堅堂婉尉常強彬彩扈啟梧畢祥笛笙統紹紳許野鹿挹珧婕婞翊防邵迢迪迥茵情惇捷理勝博喜單喬圍壺婷媚富尊就惠朝棠棟森棉焙然甥答筑結絮絡善翔肅舒評詔費貴貿超辜雅雄集順媜甯詒阿郁郃迺迴追揮琳園塘廉意慈業楚楠極楨楫楓榆煉睛筠義群裕詩詹農雷電馳馴煇煒筥詡郡連速造菩菁慎匱嘉團圖壽寧寥實廖彰愿榕榮槐榭熔熙熒睿碩禎福管筵綜綠綺綵綸緒翟臺裳誌語說誥賓趙輔銘鳴鳳郜箐箖嫺陣逸進葉慷瑤瑪嘻嬉嫻嬌寬寫慶慰樣樞標樂瑩磊範箴緯緻誼醇霆震養駐駕槿緗舖靚陳陶運道達蓉蓄蒼播器壇憲戰樺橋燕熹燃篤縈羲翰諺醒錦雕霖靜頤龍圜燁縜隆遠遣遙蓮蔭憶檀營篷繆翼臨謙謝豁賸駿檡霝適膳蕊蕙蕃環叢檳繕繡繙蟬謹鎮鵑燿遵選蕾寵龐疆繹鵲鵬麗鵰還邁懷嚴寶懸觸議飄騫騰隨臘瓏櫻譽鐸顧驅鶯鶴贔彎懿權疊讀懽孋爟驊欐驛麟隴靈靄鷹讚驤鸞籲",
		XiRadical: "艹虫豆鱼酉木田山金玉月土钅禾宀马羊牛羽忄心辶廴几",
		Ji:        `人入刀匕又久子山干丰仁什仇仍今介允公化友及夭孔少引戈支文斤日比水父片爿丘乏以付仕代令仙刊加功北仟卉占召叱外央它平幼旦永皮矛矢禾立氾丞亥伙伊伕伍休仲件任仰仳份企光全冰列刑多存式戎收早旨旬旭次此牟米艮求仵价伂托汝江池汐汎位住伴佛何估佐佑伺伸佔似但作伯低伶余佈冷別判利呀壯孝岑巫庇弟改攻攸步矣私秀良豆佟佘艾沙沈沅沛汪決沐汰沖汲汴沍沂些依侍佳使供例來佰佩侖佾侑函刻券刷到制協卓取叔受和 命委孟季岡岸岩岱岳弦或戕承旺易昌昆昂明昀昏昕昊昇枝欣爭版狀直知祀秉臥虎汯玕佼佶侄佌旻杶炅炖罕芋披注泳河沼波法泓沸油況沿治泛泊泠亮信侯俠俏保促俟俊俗俐係前剋則叛咳威孩宣弭春昭映昧是星昱染柔柏泉矜禹科秒秋穿竿風香泰芊芃玡俋昶昺蝝芝芙芽花芬芥芸芷指拯洋洲洪流津洞洗活洽派洶洛洸洵倍俸倩倆值倚倨俱倡候修倪俾倫倉凍凌剛卿孫家峻峰島弱時晉晏晃晁書核根桑益矩租秦秩秘粉素級虔託豈躬酒芮洳洺洁倢晟紘邪范茅茂茉苒英苔苑苓苯浪消浦浸海浙浮浚浩涅琉假偃偌做偉健偶偵倏副務勘動參曼唱唬夠婚寅密將崇崖崢崔崙崧崗彗悠敝救晚晤曹勗梁桿欲烯爽眾移絃處彪被袖袋豚麥苾苻浤涂玼偈偅偩偮欷欸邱能荊草荏茲茹茶茗荀荃掙涼淳淙液淡添淺清淇淋淑淞混淵涵深淨淦琅球傢傅備傑凱剴創勞場堤壹嵐幾廊弼彭敢散斯普晰晴晶景智曾棻款牌皓短稍程稅稀筍粟絨紫虛象貸酥鈍閎雯黃黍黑淀淥傌淼牋矞陀郎迷脩莎莞莘莫莒莊莓莉荷荻莆提揚港游渡湧湛渤湖湯渺湃滋渙湄湟琥傭傲傳僅催僇募勤勢匯嫁嵩愛戡新暗暉暖暄會楊歇照牒睜祿萬稜稚羨虞號裘裝該試豢鉀雉頓鼓莩渼湞渱猭幏粲豥逐萍華菱萌菲菊溶源溥溫準滄滔溪瑯瑛署僧像僑劃夥夢幕截暢歌爾種稱端箏精綱聚臧誘豪趕銀逑菇搋溱溎瑑魠蒂落萱葵葛董慢摧漳演滾漓漠漂漢滿漆漲漣漫漪滬滌億儀僵價儂儉劇劉劍增廣慮慕數暮暴槽槳歐毅獎稿稼稽稷稻篆緣諍豎豬質輝魯黎齒陟萲滸儆僾儌禠稹虢鋐遂遇蒙蒞蒲蓓澄潔潭潛潮澎潤潘儒儐劑勳曆曉曄曇樹橡機熾盧積穆糕糖臻豫鋼錄錚骸蒨潢陽遞膚蔽蔚蔓蔣蔡蓬據濃澤濁激優償儲勵嚎壕嶺嶽嶸彌懂戲戴曙燧牆矯穗糠艱虧鍚闊隸鴻蓫蓨蔘濈澲檖蕩蕉蕭濘濱濟濠濤濫濯濬濡濕斷曜朦檬歟穠糧織繒豐雙題遯蕎蕥璲禭穟襐鎵遽藏藍藉薰瀚瀝勸曦爐獻纂耀齡鏵鐌隧邃藩藝藤瀰儷驀瀴鐩蘆蘋蘇蘊歡灃籙蘭巖曬變蘩鷸蘴蘿顱灝灣`,
		JiRadical: "",
	},
	ZodiacHorse: {
		Id:        "horse",
		Name:      ZodiacHorse,
		Xi:        "乙人力三上凡也于口士大寸小川己已巾才之尹仍今介內午升天屯巴引方木火犬丙世乍乏付仕代令仙出包尼巨巧弘本未札正民玄申禾立伊休仲任仰企光全吉向宇守安寺州帆弛戌成竹羊羽而臣艮衣位何佑伸作伯佈利助吾吧妍妤宋希序廷彤村杞杉秀究良言貝走車辰依佳佩味和奉奇妹妮定宜宛尚岱幸府弦杭東林杰武直秉虎采長青佶亭信侯俊係冠勁南咸城姜威建彥柄炫為炳炬炯炮炤盈相眉科秒秋突紅紀約美貞軍韋音香芎姞姝姵迅迄巡芳芝芭芽花芬芥芷乘倍倖值倫原唐圃宸射展庭旅根桂桐桀烈真秦素純納航記訊訓財起芫芮芩邦那近若茂茉英苑苞振偌偉健國培婉寅常康強彬彩旋晨梁梵梧烽盛祥笠第符統紹紳翌處彪許章苹苻婕烺翊述迪荐草茵荏茶茗茱捷猛凱博壹媚媛崴棠棟森植焦無然登發程稅童竣策筆筑結善翔舒評費貴越超趁跑辜開閑雅集項順須寪焯焱絜軫迺荷莆猶琳琥傳勤園圓廉彙新業楚楠楨楓榆楣煤煌煥煖牒祿萬稚稠稟經綏義群裕詳詩誠詮詹農靖僊嫄椲煇煒稑豊這通連透逢逖途菩菁華菲菊獅瑞像夢嫦嫣實對廖彰榮榷榭熔熙熊熒睿種端筵箏綻綠綱綺綸維肇舞賓趙輔領颯鳳齊菀菘榞熏箐緁綦逵週逸進萱葦葉儀儂嫻寬寫樣樟標樂樑瑩稼穀稻範箴篇練緯緻蝶誼諒誕論諍賦賢賜質輝霆震頡駙逯葳萩葰緗郵運道達蓉蓓蒼儒儐勳樺橙樹樵熾燈燃積穎穆篤縈縉翰螢融諺錦霖靜頤龍蓁蒨廩燁燊遠遙蔗蓮蔬蔭燧燦燭篷績謙謝賽轅檡燡鄘遮叢檳禮穠簫繕繡謹豐顏馥儱燿鄧選薪璿寵龐穫穩證麗顗邁寶獻競籍繽耀議飄馨邈儷櫻欄躍蘋彎讀孋蘭欐麟欒讚",
		XiRadical: "艹金玉木禾虫米人月土才钅亻",
		Ji:        `一乃二又夕子山丑予仁允公友及孔少心文日月水父牛仔冉冬北台外孕平幼必旦永生田由甲皮汀氾丞再冰多好字存年收早旨旭有次牟米求忙汝江池汐汕汛汎佃但冷呂告壯妞妙孝孜孚孛岌彷役忌志忍改攻李步牢牡甫男甸矣私町艾快扭抒沙沈沅沛汪決沐汰沖汽汲汾汴汶沍沂乳亞享其函取叔受咖姓孟孤季岷岡岳往征彼忠忽念承旺易昌昆昂明昀昏昊昇服朋牧物祀忻汯旻炅育怔怖怪怕怡性拒披注泳河沼波法泓沸油況沿治泛泊泠玥勃厚叛品哈姿宣宥峒很律後思急怎怨春昭映昧是星染柵柏泉牲牴畏界禹籽泰咭昶竑肯恰恢恆恃恬拯洋洲洪流津洗活洽派洶洛洵倡凍凌哥姬孫峭峽峻徒徐恣恐恕恭恩息時晉晏晃晁書朔桑特畔留秘粉紐級酒洳洺洁畛紘翃胖胥背胡胞胤范苣悄悟悍悔悅悖浪消浦浸海浙浮浚浩涅琉參曼唱堅夠婚孰崇崎崢崑崙崧崗庸得從御悉悠敝救教啟晚晤曹勗望烯爽牽犁瓷產略畢異眸粒絃細羞被袖袋浤浡涂娸婭胭脈能悽情惜惟悸惇涼淳淙液淡添淺清淇淋淑淞混淵涵深淮淨淦球勝喜單喬場堤孳富尊幅幾復循悲惠敦敢散普晰晴晶景智曾期朝椅棋犀甥畫皓粥詠距鈕閎雯黍黑淀淥惢淼甯郁莎莫慨惶愉提揚港游渡湧湊湛湘渤湖渭湯渺湃滋渙湄湟琪琦匯嵩微愚意慈感想愛愁愈愍暗暉暖暄會楊照煦當畸祺羨裘鉀鉅鈿預愃渼湞渱粲碁造慎慌慄慍愧溶源溥溫準滄滔溪署嘉壽夥徹慇態暢爾犒監福粹精臺誥郜逑溱溎郭慷慢慣慚漳演滾漓漂漢滿漆漲漣漫漪滬滌嘻增墩德徵慶慧慕慰慾數暮暴槽畿諄賣醇靠駐魯黎陟滸鋐陳遊遇蓄蒞憐憎憤澄潔潭潛潮澎潤潘冀器學憲憩戰曆曉曇機燕諮謂醒錄潢陽隆膠憶憾擋濃澤濁澳激孺嶺應懂懇懋曙檔糠臨鍚闊隸駿鴻濈澲膳蕙蕩濘濱濟濠濤濫濯濬濡濕環曜繒釐雙題騎鵠騏遲臆薑薇濾瀑嚥壞曝犢疇疆矇懷瀚瀝嚴懸曦朧纂騰麵藩藤瀰巍籐瀴懼孿疊鑑籙戀曬纖變隴灝灣驥`,
		JiRadical: "",
	},
	ZodiacSheep: {
		Id:        "sheep",
		Name:      ZodiacSheep,
		Xi:        "乙丁乃几力丸凡久也士寸小川己巳丹之尹云允午升少屯巴引木火丙世乏充尼巧平弘本玄田由甲禾立先印名合回圳地在圭妃宇守安弛曲朵竹老而臣自至兌克助均圻妙妍妤宋宏廷村杞步甫男私秀豆里妘町亞兔典和坤妹定宜宙宛尚居屆岳幸延弦杭東果林杰松炎秉舍青芋亮信勇勉匍南品垂室宥封巷建扁柵柄柳炫為炳盈科秒秋風芎垚迅巡芳芝芽芹花芸芷原圃埔堉娜娟娥家容庭桂桃殊烈畔秦秩航起軒馬芫邦那迎近若茂苗英苑堅基培婉專庶強梁梵梨烽畢笛章苹埜婕娸述迪草捷凱博喜喬媚富弼棟森棣棋焚無登發程稅童筍筑粟舒貴開閑閎雅雄集順茜焱莞莆圓廉彙敬業楠楓楣煉煥萬稜稚節筠粱義詩跳靖馳馴郅楙稑粲豊通速逐逢菁華菊嘉境夢榮榴槐熙睿端筵精豪輔颯魁菀菫菘墉墐槊榞箐箖逸進葉嫻寬樂毅稼穀範蝶豎賢逯萩槿道遂達蓉蒙蒼樺橙樹橡橋燕篤臻螢豫霖靜蓁燁遠遙遛蔗蓮蔬燭穗篷臨駿檡適檬簧豐蹕蕓鄰選薪穫簾證韻邁薰躂馨隧藝躍蘋驊蘭驛艷驥",
		XiRadical: "金白玉月田豆米马禾木人艹鱼亻",
		Ji:        `一二刀匕大子巾干弓丑予仁什今天夫太孔心戈支文斤日比水片牙牛王爿主仔代冬刊加功北占古召叱央孕它巨市布弗必旦永玉生皿矛矢示丞光冰列刑吉夷好字存帆年式戎戌成早旨旭有次此牟衣求忙忖托汝江池汐汕汎佔但作伯伶冶冷別判君告呀坎壯夾妞孝孜孚孛希庇弟彤形忘忌志忍李杉牢牡系車辰酉礽忱快忸扭抒沙沈沅沛汪決沐汰沖汽汲汴汶沍沂狄玖乳些享依例函刻券刷到制協奉奇奈姓孟孤季帛忠忽念或戕承旺易昌昆昂明昀昏昕昊昇服朋枝欣版牧物狀直知社祀祁糾初采金長忡忻汯玕佌旻炅罕育怔怖怪怕怡性怩拒注泳河沼波法泓沸油況沿治泛泊泠狐玨玟玫玥表係前剋則勃厚奕奏奎奐姿孩宣帝帥弭彥思急怎怨春昭映昧是星染柔架柏泉炬牲牴矜祉祈祇穿竿籽紂紅紀紉紇約紆美羿衫泰怜玡玦玠昶柰祅紈十股肴肯恍恰恢恆恃恬指拯洋洲洪流津洗活洽派洶洛洸洵珊玲珍珀玳倡修凍凌准剛唐套奘奚姬孫席師恣恐恕恭恩息時晉晏晃晁書朔朗栩特矩祕祐祠祟祖神祝祚秘紡紗紋素索純紐級紜納紙紛翁袁袂衽託躬酒恂洳洺洁珅倧扆祜祓紘紓衿衾邪胖胥背胡胞胤范茅苣苓悄悟悍悔悌悅悖浪消浦浸海浙浮浩涅班琉珮珠副務勘動曼唱唬國婚孰將崇常康張彬彩彫悉悠救教旋晚晤晨曹勗望桿烯牽犁瓷產眸祥祭絃統紮紹紼細紳組終羞羚翎聆彪被袒袖袍袋責悒浤浡涂玼珣珩婭紵紾袗胭脈能悽情惜惕惟悸惇掛採捺涼淳淙液淡添淺清淇淋淑淞混淵涵深淮淨淦猛琅球理現剴創勞勝場堤壹孳尊幀彭悲惠敦斯普晰晴晶景智曾期朝棕椅牌犀甥番皓短等結絨絕紫絮絲絡給絢翕裁裂視詠診費距鈕須黍黑淀淥琁琇珺淼牋甯矞絪絜軫陀郁脩莎莫莊莉慨惶愉提揚港游渡湧湊湛湘渤湖湯渺湃滋渙湄湟猶琪琳琥琴琛琦琨勤勢匯愚意慈感想愛愁愈愍戡暗暉暖暄會楊照牒猷祺祿禁稟經絹綏羨群裟裙補裘裝裕試鉀鉅雉零預愃渼湞渱琮琬琰煒絻郡造慎慌慄慍愧溶源溥溫準滄滔溪獅瑚瑟瑞琿瑙瑛瑜署劃壽寥廖彰慇態截暢犒監碧禎福禍粽綻綰綜綽綾綠緊綴網綱綺綢綿綵綸維緒緇綬臧裴裸製褚誥趕閨滕郜逑愫溱溎瑄瑋瑗綪緁緆緋綖綦裱魠郭慷慢慣慚漳演漓漂漢滿漆漲漣漫澈漪滬滌瑤瑣瑰劇劉劍增墩審幟影慶慧慕慰慾暮暴槽槳獎瑩締練緯緻緘緬編緣緞緩緲翩褐複褓褊諄賣質輝醇靠魯黎齒摎滸漻褌褋褙陳遊蒞憐憎憤澄潔潭潛潮澎潤潘璋璃瑾璀冀劑勳奮學憲憩戰曆曉曇熾禦穎縑縈縣縝縉褪褲褫諮醒錄頤潢璇璉縕錼陽隆膠蔣憶憾濃澤濁澳激獨璟璞勵孺彌應懂懇懋戲戴曙曖牆矯禧禪糠績繆縷總縱縴縵翼聰襄褸鍚闊隸鴻濈潞澲蟉襁膳蕙蕩濘濱濟濠濤濫濯濬濡濕獲環璨斷曜璧禮穠織繕繞繚繡繒釐顏題鵠蕥際鄭遲臆薑濾瀑璿嚥壞曝牘犢璽疆矇禱繫繹繩繪繳襠襟識藏懷瀚瀝瓊勸寶懸曦朧獻繽繼耀齡繾瀰瓏櫻纏續瀴懼瓔孿彎懿禳襯鑑灃籙鷚戀曬纓纖襴鷸隴灝纘灣纜`,
		JiRadical: "",
	},
	ZodiacMonkey: {
		Id:        "monkey",
		Name:      ZodiacMonkey,
		Xi:        "乃人力丈上久于千口土士大子寸巾中丹尹云井仁仍今介元允內公化升壬天夫太孔少手水王主以付仔仕代令兄凸北可右司台句巨巧左本母民永玄立汀丞仿伊伍休仲任企光全匡吉吏同向名合因回在圭如字存宇守安曲次百竹羽而臣自至行衣求汝亨位何佐佑似作伯佈冶吾呈含吟坊均壯妙妍妤孝孜宏巫希序形攸杏材村杉每言貝走足辰妘技沛沐沂玖亞享京依佳來佩侑兩典函周坪夜奉奇孟季定官宜宙尚居幸府征承放於杭東果林松直知采長雨青政汯佶注泳河法治亮信俠促俊係冠勁南厚品垣城奕奎客封帝彥昱柱柏泉爰盈紅美衍貞軍音首泰俅垚姵芳芸持拱拯拾洋洲洪洞玲珍倍倆倚俱候修倪倫倉唐哲員堉娟孫宮容宸展師庫座旅時桓桂桔栩桐格桃真祐站紡純級納袁記訓起馬高珂倜旆紝紘紓若茂英振海浚浴珮乾偌做偉健偵偯凰區商國域堅基堂培尉專常康彬得御徠教啟敏族旋晨梵梧祥符笙統紹訪許責苙浤浡浯珩埼婕娸茵茴荏捷授涼淳清淇淋涵深淦球理備傑勝博喜喻喬圍堪堤堡壺婷媚媛復循朝棠棟森棉登竣策筆筑善舒詠評詞証詔賀貴越超開閎雲順馮淥琇喨婼媜淼詒雱郁荷揮援揚湘滋渙湄湲琳傳僅勤嗣園圓塘彙新楚毓祿筠絹義群裔裘裕詩詮詹農靖湜筥郡萊溶滇僖僑嘉圖境嫣實彰徹旗睿禎綽綠綺綢綿綵維聚肇臺舞語誥賓領颯魁菉瑋墉漳漢滿瑤瑪億儀儂增寬履德徵慶樣樟樂緯緻衛誼諒諄調論賞賦賢質輝震滸槿霈蓉蒞蒲蓀蓓潔儒儔儐勳學樺橙橋穎穆縈縉興衡諺謀諮霖頤龍蒨澐璇叡圜隆隄擇濃澤償儲嬪孺檀檜營績總繁聯臨謙謝謄豁賽趨駿黛霝龠濱濟濤濬濡環璨檳禮繕謹豐顏儱騏薔薇寵繹識證贊辭韻麗嚴寶竇競籌繽繼纂覺議警譯騰齡瀧藝瓏儷櫻續覽譽露蘋讀孋驊灑驗麟欒纕隴靈靄讚灤驤驪",
		XiRadical: "木禾金玉豆米田山月水人氵亻",
		Ji:        `刀匕山干弓丰什午屯引戈支斤比片牙爿丘仙冉刊加功卉占古召叱它平幼弘弗田由甲申皮矛矢禾亥再列刑式戎旨此米艮托佃佔別判利助呀岑庇弟甫男甸私秀良豆些例刻券刷到制協卓和委岡岸岩岱岳弦或戕昕枝欣爭版狀盂祀秉虎金忻玕佌旻杶炘炅炖罕泓前剋則勇咳威孩弭思柔畏界矜科秒秋穿竿要風玡蝝芽指剛卿家峻峰島朗核畔留矩租秩秘粉素虔託豈豹躬酒配釗邪近茅浪副務勘動唬婚寅密將崇崖崢崔崙崧崗庸張強彗梁桿烯略畢異盛眾移粗細處彪袖豚釧麥玼邱能掙淺淨琅傢凱剴勞單場壹嵐幅廊弼彭斯普晰牌畫番短稍程稅稀粟絨紫虛覃象貂費酥鈔鈞鈍黍牋矞陀郎迷莫莊莉提湧渭琥傭催勢嫁嵩戡牒當畸盟睜稜稚虞號裝該試豢鉀鉛鈴鈿雉頓鼓猭幏粲豥鈺逐準獅瑯署像劃截監福種稱箏粹精綱臧誘豪貌趕銀銅銘鳴鳳搋瑑魠慢摧演漫劇劉劍慮暮槽槳毅獎盤稿稼穀稽稷稻篆緣諍豎豬銳鋒駐魯黎齒禠稹虢陳遂蓄蒙潛澎潘劑戰曆曉曇橡熾盧積糕糖臻謂豫醒錢鋼錫錚錦錕靜骸鴛陽遞膚蔣擋據勵嚎壕嶺嶽彌戲戴曙檔燧牆矯穗艱虧鍵鍾鍛鴻蓫檖鍼濠斷曜朦檬糧織繒醫鎮題遯蕥璲禭穟襐鎵鄭薪薑濾嚥廬曝牘疇疆穫穩鏡鏈鏢鏗靡類鵬繸襚鏞鏔遽藏勸曦爐獻耀鐘麵鐌隧邃藩鐵鐸鶯鶴鐩蘆蘇彎疊鑑灃巖曬鷸鷹蘴顱鸚鸞`,
		JiRadical: "",
	},
	ZodiacChick: {
		Id:        "chick",
		Name:      ZodiacChick,
		Xi:        "乙乃二下上凡千口土士大小川己巳巾才丑中之今午升少引日牛世仕巧市布平民生用田由甲禾立光兆吉向回地如宇守年早曲竹米羊羽臣自佑告坊均妞妤廷杏甫男秀言豆辰里町亞佳卓味和定宜宙尚居岡岸岩岳延昕杭果林竺舍長非佶亭信勁宣峒建彥扁是星盈眉科紅紀虹計赳軍革韋音飛芊垚姵畇紃迅巡芳芝原唐員圃埔堉家容宸展峰席庫庭料畔真秦紐耕航記訓財貢起軒邗近若苗苑振浦浩珮乾堅堆基堂培婉專崎崖崢崔崗常康強彬彩彫旋旌晨曹梁梵笠笛笙翌蛇許章頂苹婕翊述迪草凱博堤壹富嵐彭登發皓程童筆筑粟善舒詠評賀貴超量順郎迺莆揚圓塘廉暉業楚當筠義詳詩農雋雷邽畹竫粲豊通連造逢菩萍菁華著菲菊境對彰榮睿種端筵精綠綱綵綸維翡翟肇臺誥賓輔閨颯鳴鳳墉墐陣部都逵進萱葦葉葡董儀增嶝幟穀稻範篇緯編緹誼諒談論豎賞賢賜輝霆震郴逴萩運道達蓉蒼播導彊曆暹曄橙積穆縈翰臻融親諺輯雕霖霏龍蓁廩諟隆遠遜遙蓮蓬勵嶺嶸穗翼臨舉鄘適蕃曜糧繙蟬謹豐馥鄧遵選薪蕾薇疇疆穩韻麗還藍薰寶競耀譯馨躍麟艷",
		XiRadical: "米豆虫木禾玉月宀山艹金钅",
		Ji:        `一人刀力匕兀子干什仇元允友天夫太心戈手支文斤月木比片牙犬王爿代充兄刊加功北卉占卯古召叱央它弘弗必戊旦本玉申矛矢石示亥伏仰先共列刑印式戎戍成旨旭有朴此旮忙托佔但伯伶兌克免別判利助呂君吠呀坎壯夾宋宏庇弟形忌志忍杜究系酉艾快忸抑狄狂玖些例兔刻券刷到制協奉奇弦忠忽念或戕旺易昌昆昂明昀昏昊昇服朋東枝松欣武版狀直知祀金青忻玕佌旻炘炅罕肖育怔怖怪怕怡性泓狗狐玟玫玥亮前剋則勇勉咸品城奕奏奎奐威帝弭思急怎怨春昭昧柔柯柏柳畎相矜秋穿突竿美狘玡峸肴肯芙芽恰恢恆恃恬指狩珊玲珍珀玳倡冤凍剛卿哥哭奚娩娥恣恐恕恭恩時晉晏晃晁書朔朗桓柴烘留矩秘素虔託躬酒配釗高狨珋宬彧晟毧邪迎胖胥胡胞胤茅茂苓茆悄悟悍悔悅悖挽班琉珠冕副務勘動曼啞唱唬國域婚將悉悠啟晚晤勗望桿烯異盛羚聊聆袖責悒浤玼馗胭脈能悽情惜惟悸惇淺清猜猛琅球理現剴創勞勝喜喬場崴弼惑悲惠斯普晰晶景智曾期朝棟棚然牌短絨紫費越鈞雄雯茙茯淢琁珺斌棫牋甯矞羢陀郁脩莫莊莉荻慨惶愉提減湖猶猴琪琥琴琦琨勤勢愚意慈感想愛愁愈愍戡暗暖暄會楊照牒猷群裝試誠鉀鈴鉚雉零郕愃猭琰稢鉞郡菟慎慌慄慍愧滅準獅猿瑚瑟瑞瑙瑛瑜署劃嘉慇態截暢榕榴獄碧綻臧趕輓銀銘滕獂瑊瑋瑀戫魠逸慷慢慣慚漫瑤瑣瑪瑰劇劉劍慧慰慾暮槽樓槳獎瑩箴糊蝴質銳鋒駐魯齒葴羬鄉憐憎憤潛璃璀劑勳器奮憲憩戰曉曇熾燃醒錢默璇璉諴醍駥陽膠蔣憶憾獨璞嶽彌應懂懇懋戲戴曙牆矯績鍾鍚隇鍼膳蕙獲環璦璨斷璧織繕繒鎮題蕕蕥鎦鄭臆薑獵璿嚥壞曝牘獸璽矇識鵬藏懷瓊勸嚴懸曦朧獻鐘騰齡騮瓏櫻懼瓔彎懿權鑑戀曬纖鷸鷢隴灣`,
		JiRadical: "",
	},
	ZodiacDog: {
		Id:        "dog",
		Name:      ZodiacDog,
		Xi:        "丁人力三上千口土士夕大寸小山巾丹尹云今介允內公午升太少心月火王丙世丘主以付仕令可右司巨弘必戊玄玉生用立伊休任仰企全匡吉同向名合地在圭如宇守帆成有百臣自至艮衣位住伴佐佑伸伯伶佈呈坊均壯妙宋宏希志忍良走里亞依佳佩侑典卓味坪坡坤夜奉定宜宙岡幸府忠念朋東武虎采門青忻佶侄亞佳卓味和定宜宙尚居岡岸岩岳延昕杭果林竺舍長非佶肱肴肯芯芸芷恆恬玲珍倚倫原堉家容庫庭恩朗烈純紜訓起軒馬芮珂倢紘迎苗悟悅珮珪乾偉健國堅基堂培寅尉專常彬彩敏烽盛祥笙統紳處彪偲埜婕婧情惇捷清備傑凱勝喻堡媚富惠期朝然童舒越閎斌焯郎郁愉揮琥傳僅園塘意慈愛愈楠榆煉煥萬筠絹裕誠靖馳馴鼎媱煇菁華瑯瑟琿瑜境嫣寧對彰旗榮睿箕綻綠網綺綸維綬肇臺舞誌輔箐逸萱瑪億儀寬慧慰瑩練緯論賞輝駐駕僾褌運蓉蒼憬儒器壇奮憲燕篤縈縉螢融親諺豫靜蓁蒨曈燁遠憶優營燦績臨舉駿黛蕙環璦叢騏蕾璿繹韻懷懸獻闡騫騰繻藝驊驥",
		XiRadical: "鱼豆米宀马金玉艹田木月禾水人氵钅亻",
		Ji:        `刀匕子干弓丰之仁什尤引戈支文斤日曰木比氏水片牙爿代冉刊加功北卉占古召叱它弗旦本民永田由甲申矛矢禾丞光冰列刑安式戎早旨旭次此羽求托汝江池汐汎佃佔但冷別判利呀庇弟束李杏材村杜杖杞杉甫男甸私秀豆貝辰酉把批沙沈沅沛汪決沐汰沖汲汴沍沂狄些例函刻券刷到制協和委妹季岷或戕承旺易昌昆昂明昀昏昕昊昇杭枋果杳枝析枚欣版狀直祀秉長雨汯玕佌旻炘炅九罕注泳河沼波法泓沸油況沿治泛泊泠狐前剋則弭律春昭映昧是星昱柿染柱柔架柯柑柚查柏泉畏界相矜科秒秋穿竿羿貞酋酊飛香泰玡昶芽指拯洋洲洪流津洗活洽派洶洛洵倀倡凍凌剛宸徐恭時晉晏晃晁書校核桓根桂桔栩栗桑柴桐桀格桃株烏畔留益真矩租秦秩秘翅翁託財貢躬辱酒配洳洺洁栖翃邪近范茅振浪消浦浸海浙浮浩涅狹偵副務勘動曼唱婪婚崔帳庸張強得從救晚晤晨曹勗梁梓梵桿梧梗梭梅梨烯略畢異移細翌翎習袖責貫酗雀鳥鹿麥浤涑涂玼珝翊悵涼淳淙液淡添淺淇淋淑淞混淵涵深淨猜猛球剴勞博單場壹就幅弼彭復斯普晰晴晶景智曾棕棘椅棟棹棋植棉棚欽牌畫番皓短稍程稅稀粟絨紫翔翕詠貽費賀貴買貿酥雅雄集雲順黃黍黑茜淀淥媜崷棨淼牋矞陀迷莎莫莊莉提揚港游渡湧湛湘渤湖渭湯渺湃滋湄湟猶琳僇勤勢匯微戡新暗暉暖暄會榔業楷極椰楊楨楓牒當畸祿稜稚義羨群裘裝試詩資賈賄賂農酬酪鉀鈿雉鼓渼湞渱煪詡溶源溥溫準滄滔溪署劃寥實廖徹截暢榜榕構榛榴槐熊禎福種稱精翠翡翟臧裴誘賓趕酷領鳴鳳逑溱溎嵺嶍榞槙魠慷慢漳演漓漂漢滿漆漲漣漫漪滬滌儂劇劉劍增廣德暮暴樣樟標槽模槳樑獎稿稼稽稷誼豎賠賦賢賣賜質醇醋震魯黎齒憀摎滸漉漻槢樛潁熠熤稹翦陳蓄蒞澄潔潭潛潮澎潤潘劑勳戰曆曇橙橫樹橋樵機歙熾積穎穆篩糕糖翰翱翮臻諦謂賴醒錄霖頭頤鴦鴒鴛龍遒潢醐醑鋹鴗陽蔣擋濃澤濁激獨嬰彌戲戴曙檀檔檢牆矯穗糠繆翳翼褶賺賽購鍚闊隸鴻鴿鄔蔍濈澲嚁簏韔蕩濘濱濟濠濤濫濯濬濡濕斷曜檳櫂穠糧繒翹翻豐蹟醫雙顏題鵑鵝鵠蕥燿豂轆轇鄭遺薪薑濾瀑嚥寵龐曝櫚牘疇疆穫穩識贊醮靡類願鵡鵲鵬麒麗麓騛鵰藏瀚瀝勸嚨寶曦朧耀醴麵齡藋瀧曨櫳顟飂鶖鶧藩瀰瓏儷櫻欄躍顧鶯鶴麝瀴趯鶼蘋蘇瓔彎權疊籠襲龔灃籙譾鷚龕曬纓顯鷥麟鷸鷲隴鷹鷺蘴灝鸐酈灣鸕鸚鸞鸝`,
		JiRadical: "",
	},
	ZodiacPig: {
		Id:        "pig",
		Name:      ZodiacPig,
		Xi:        "二口士女子寸小山尹云壬孔少方月木水牛四充北卯未民永生用田由禾立汀丞亦亥兆再冰吉同合回好如字存宇守安寺曲竹米羊臣西求汝江亨兌免助呈告妞妙妍妤孝宏序材牡甫男私秀豆里妘沂乳亞享侖兔函和固委妹孟季定宜宙尚居岳庚承東果林松秉竺舍金雨青芋芍泳河法泗治勉勃厚品姜姿屋扁柵柄柳盈相眉科秒秋食泰芳芹芸洋洲津洞洛兼凍卿原圃娩娥容峰料朗案根桂栗桐桃株特畔畜留秦窈財酒桉若茉苗苑茆浪浦海涓浴浩動寅專崎崙崧康旌梁梓梅笙粒章鳥麻浤婕婞娸婭茵茲茶捨涼淳清淋淵淅涵深淦博喜喬媚富廊棟森棲棣甥登發童策筍筑粟善舒詠鈕鈞閑閒閎雅雄集雲黍黑茜淥淼莘莆揮湘湖廉敬新業楣毓稚義鉛鉉鈿雷靖飽豊萍華萊菊溢溶源溫嘉實榮榭睿種箕精翡翟肇舞輔銀銘菉溱溎榞箐銚嫺葵葦葉葡演漾滿嫻嬌寬廚廣樣樟樑穀稷稻箱篆篇諄豎醇鋅鋒養萩槿鋐鋆蓉蒙蒲蒼澄潔潭潤學樺樹橡橋糕羲臻豫錠錄錐霖霍頤默蓁澐燊蔬蔭濃澤孺嶸瞳糠臨鍾鞠鴻檡濱濟濛濤濬濡叢檬櫂穠糧謹豐鎮馥薪蕾穫穩鏗韻鵬瀅鏞藍薰瀝馨瀠藩藝藤鐸露鶴藺權鑑蘭麟鑫",
		XiRadical: "豆米鱼水金玉月木人山土艹氵亻",
		Ji:        `一乙几刀力匕三上凡也土大川己已巳干弓之仁什今天太巴引戈支斤日比片牙王爿主乏代刊加功包卉占古召叱央它市布平弘弗旦玉申皮矛矢石示氾光列刑危夷妃州帆式戎早旨旭朵此虫血衣圮托汎伸佔但伯伶别判君呀坎壮希庇廷弟彤形杉系邑礽玖些依例刻券刷到制协卓卷呻坤奇奈宗宛延弦或戕旺易昌昆昂明昀昏昕昊昇枝欣版状直知社祀祁纠初采长忻玕佌旻炘炅罕泓泡泛玫表亮侯係前剋则勇垣奐宣巷帝帅建弭彦春昭映昧是星昱架柏矜祉祈祇禹穿竿紂红纪纫紇约紆羿虹衫风玡玠柰祅紈迅巡芝芽指珊玲珍珀玳倡候修刚奚孙家差席师庭时晋晏晃晁书核栩矩砷祕祐祠祟祖神祝祚纺纱纹素索纯纽级紜纳纸纷翁蚩袁袂衽讯託起躬珅倧扆祜紘紓衿衾邪邦那迎返近范茅苓班琉珮珠乾健凰副务勘曼唱唬婉婚将崇常张强彬彩彫旋晚晤晨曹勗桿烯祥票祭絃统扎绍紼细绅组终羚翌翎聆彪蛉被袒袖袍袋责玼珩埏紵紾袗邵述迦迪能情掛採捺涎浅琅球理现剴创劳喉场堤堠媛嵐巽帧弼彭斯普晰晴晶景智曾棕牌番短结绒绝紫絮丝络给绚絳翕蛟裁裂视诊费贵须邰琁珺牋矞絪絜軫陀郎郁送迷迺莫庄莉提扬琪琳琥琴琦琨勤势园戡暗暉暖暄会杨枫照煜牒祺禄禁经绢绥群蜀蛾蜕蜂裟裙补裘装裕试钾雉零琬琰琝媴絻郡通连速造透逢途準猿瑚瑟瑞瑙瑛瑜署僎划寥廖彰截畅碧禎福祸粽绽綰综绰綾绿紧缀网纲綺绸绵綵纶维绪緇綬臧蜜蜻裳裴裹裸製褚豪宾赶闽溒瑄瑋榬綪緁緆緋綖綦蜒裱魠郭都週逸进慢漫瑶琐玛瑰剧刘剑增审影暮槽桨毅奖莹稼缔练纬緻缄缅编缘缎缓緲緹翩蝴蝶褐复褓褊诞赏质辉驻鲁齿摎漻褌褋褙陈乡运游道达违遁撰潜澎璋璃瑾璀剂勋战历晓曇炽御縑縈县縝縉萤融褪裤褫讽醒骇璇璉縕螈錼阳邹远逊遣遥递蒋璟璞励弥戏戴曙墙矫禧禪绩繆缕总纵縴縵翼襄褸辕鍚鄔蟉襁适迁环璦璨断曜璧织缮绕繚绣繒蝉顏题蕥鎱际郑邓选迟薑璿嚥曝牘璽疆祷繫绎绳绘缴襠襟识赞譔还迈邀藏琼劝曦繽继耀腾龄繾饌邈瓏樱缠续边瓔弯禳衬鷚晒缨纤襴鷸蛮纘逻湾缆驪別壯協狀糾長則帥彥紅紀紉約風剛孫師時晉書紡紗紋純紐級納紙紛訊務將張強統紮紹細紳組終責淺現創勞場幀結絨絕絲絡給絢視診費貴須莊揚勢園會楊楓祿經絹綏蛻補裝試鉀連劃暢禍綻綜綽綠緊綴網綱綢綿綸維緒賓趕閩進瑤瑣瑪劇劉劍審槳獎瑩締練緯緘緬編緣緞緩複誕賞質輝駐魯齒陳鄉運遊達違潛劑勳戰曆曉熾禦縣螢褲諷駭陽鄒遠遜遙遞蔣勵彌戲牆矯績縷總縱轅適遷環斷織繕繞繡蟬題際鄭鄧選遲禱繹繩繪繳識贊還邁瓊勸繼騰齡櫻纏續邊彎襯曬纓纖蠻邏灣纜`,
		JiRadical: "",
	},
}

func GetZodiacById(z string) *Zodiac {
	for _, v := range zodiacList {
		if v.Id == z {
			return &v
		}
	}

	return nil
}

// GetZodiac ...
func GetZodiac(c chronos.Calendar) *Zodiac {
	z := chronos.GetZodiac(c.Lunar())
	if v, b := zodiacList[z]; b {
		return &v
	}
	return nil
}

func (z *Zodiac) zodiacJi(character *model.Character) int {
	if strings.IndexRune(z.Ji, []rune(character.Ch)[0]) != -1 {
		return -3
	}
	return 0
}

func filterZodiac(c chronos.Calendar, chars ...*model.Character) bool {
	return GetZodiac(c).PointCheck(3, chars...)
}

// PointCheck 检查point
func (z *Zodiac) PointCheck(limit int, chars ...*model.Character) bool {
	for _, c := range chars {
		if z.Point(c) < limit {
			return false
		}
	}
	return true
}

// Point 喜忌对冲，理论上喜忌都有的话，最好不要选给1，忌给0，喜给5，都没有给3
func (z *Zodiac) Point(character *model.Character) int {
	dp := 3
	dp += z.zodiacJi(character)
	dp += z.zodiacXi(character)
	return dp
}

func (z *Zodiac) zodiacXi(character *model.Character) int {
	if strings.IndexRune(z.Xi, []rune(character.Ch)[0]) != -1 {
		return 2
	}
	return 0
}

func (z *Zodiac) GetXiRadical() string {
	return strings.Join(strings.Split(z.XiRadical, ""), "、")
}

func (z *Zodiac) GetXiArray() []*model.Character {
	xi := strings.Split(z.Xi, "")
	var chars []*model.Character
	for _, v := range xi {
		char, ok := Character[v]
		if ok {
			chars = append(chars, char)
		}
	}

	return chars
}

func (z *Zodiac) GetJiArray() []*model.Character {
	ji := strings.Split(z.Ji, "")
	var chars []*model.Character
	for _, v := range ji {
		char, ok := Character[v]
		if ok {
			chars = append(chars, char)
		}
	}

	return chars
}

func (z *Zodiac) GetZodiacScore(chars ...*model.Character) int {
	score := 80
	for _, v := range chars {
		score += z.Point(v)
	}

	return score
}