# 基于安企CMS + Fate 二次开发的起名网站安装教程

## 应用到的开源项目：

1. [AnQiCMS](https://github.com/fesiong/goblog)  
2. [Fate](https://github.com/babyname/fate)

## 安装方法：

1. 下载 [anqicms-linux-fate.zip](https://github.com/fesiong/goblog/releases/tag/fate)  
2. 安装步骤按照AnQiCMS的安装步骤进行，[AnQiCMS安装教程](https://www.anqicms.com/help-basic/210.html)  
3. 登录网站后台，在功能里找到备份与恢复功能，找到备份文件[20240908-qiming]，点击恢复。由于备份文件较大，因此恢复时间可能会超过1分钟，为了确保恢复成功，建议在点击恢复后，等待10分钟左右，点击右上角重启按钮，重启网站。  
4. 恢复并重启网站后，来到模板设计里，点击启用[起名模板]。

完成以上步骤后，即可得到一个完整的起名网站。该网站的起名功能仅集成到了前台，后台管理功能未实现。

## 起名网站的功能：

1. 八字起名  
2. 古籍起名  
  (论语、大学、中庸、诗经、周易、楚辞、尚书、道德经、唐诗、宋词、三字经、千字文、美好成语)  
2. 姓名测试  
3. 生辰八字测算  
4. 起名字典  
5. 百家姓  
6. 生肖起名  

## 界面截图：

首页
![index](example/index.webp)

起名结果
![index](example/naming.webp)

姓名测试
![index](example/detail.webp)

八字测算
![index](example/horoscope.webp)