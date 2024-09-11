<div align="center"><a name="readme-top"></a>

<img height="180" src="docs/anqicms.svg" />

[更新日志](./CHANGELOG.md) · [English](./README-en_US.md) · 中文

# AnQiCMS

# 基于安企CMS + Fate 二次开发的起名网站 [安装教程](usage.md)

</div>

## 介绍

安企内容管理系统(AnqiCMS)的前身是GoBlog，一款基于 iris 框架，使用 golang 开发的简洁版个人博客系统

> GoBlog是一个由 golang 编写的开源个人博客系统，界面优雅，小巧、执行速度飞快，并且对seo友好，可以满足日常博客需求。它的使用很简单，部署非常方便，pc和移动端自适应，页面模板使用类似blade模板引擎语法，上手非常容易，适合个人博客使用。

安企内容管理系统(AnqiCMS)，是一款基于 iris 框架，使用 GoLang 开发的企业内容管理系统。它部署简单，软件相对于传统的PHP开发的内容管理系统更加安全，界面优雅，小巧，执行速度飞快，使用 AnqiCMS 搭建的网站可以防止众多常见的安全问题发生。AnqiCMS 的设计对SEO友好，并且内置了大量企业站常用功能，对网站优化有很好的帮助提升，对企业管理网站一定程度上提提高了办事效率，提高企业的竞争力。

AnqiCMS 除了适合做企业站，也适合做营销型网站、企业官网、商品展示站点、政府网站、门户网站、个人博客等等各种类型的网站。

AnqiCMS 支持 Django 模板引擎语法，该语法类似 blade 语法，可以非常容易上手模板制作。网站模式支持 自适应、代码适配、PC+mobile 独立站点 三种模式，根据不用需求，可以选择适合自己的搭配方式来建站。

我们的追求：让天下都是安全的网站。

我们一直朝着网站安全的方向前进，让 AnqiCMS 为你的网站安全护航。

欢迎您使用 AnqiCMS。

## 快速开始

[下载最新的 AnqiCMS](https://github.com/fesiong/goblog/releases)  
[安装一个新的站点](https://www.anqicms.com/help-basic/210.html)  
[查看模板使用教程](https://www.anqicms.com/manual)  
[查看后台使用帮助](https://www.anqicms.com/help)  
[查看接口文档](https://www.anqicms.com/anqiapi)

> 温馨提示：大多数文档的编写语言为中文，如果您是英文用户，请使用浏览器自带翻译功能自行翻译。

## 网站特色功能

- 自定义文档模型
- 自定义页面导航
- 富文本、Markdown 编辑器支持
- Webp 图片支持
- 多模板自定义支持
- 多站点支持
- 数据统计详细记录
- 自定义伪静态规则
- 多个搜索引擎主动推送
- Sitemap管理
- Robots.txt管理
- 友情链接管理
- 内容评论管理
- 自动锚文本功能
- 网站留言管理
- 关键词库管理
- 内容素材管理
- 邮件提醒功能
- 文章采集功能
- 文章组合功能
- 文章导入功能
- 自定义301跳转功能
- 网站内容迁移功能
- 静态页面功能
- 自定义资源存储
- 用户管理
- 用户组管理
- 小程序支持
- 全文搜索支持
- 备份与恢复
- 文章自动配图支持
- AI自动写作功能
- 定时发布/更新功能
- 防采集干扰功能
- 图片水印功能

## AnQiCMS 发展历程

- 最新
  > 我们一直在努力，不断的探索，持续迭代优化，争取给用户们有更好的使用体验。
- 2024年 5月 1日，v3.3.5 发布
  > 支持图片水印功能
- 2023年10月24日，v3.2.5 发布
  > 支持多语言翻译功能，支持 Markdown 编辑器
- 2023年 4月15日，v3.1.1 发布
  > 接入AI自动写作功能
- 2022年12月 5日，v3.0.0 发布
  > 开始支持多站点模式，简化和降低了 AnQiCMS 的部署难度，新增更多丰富的企业站常用功能
- 2022年 5月30日，v2.1.0 发布
  > 正式更名为 AnQiCMS，标志着 AnQiCMS 已具备常用的内容管理系统必备功能
- 2021年 2月16日，v2.0.0-alpha 发布
  > 开始逐步由单纯的博客功能，过度到更全面的内容管理系统，逐步开发并完善企业站功能
- 2021年 1月21日，GoBlog v1.0.0 发布  
  > 博客完善版，在基础版的基础上，增加了后台管理、seo功能等。
- 2020年12月 1日，GoBlog v0.5 发布  
  > 重构版本，采用iris框架重写，减少技术栈，改用iris自带的template模板引擎。实现了最基础的博客功能。
- 2019年11月19日 GoBlog v0.1 发布   
  > Gin版本，前后端分离，后端使用go、gin、gorm，前端使用Next.js。

## 使用的包

- [gorm](https://github.com/go-gorm/gorm)
- [iris](https://github.com/kataras/iris)
- [jwt](https://github.com/golang-jwt/jwt)
- [sego](https://github.com/huichen/sego)
- [gorequest](https://github.com/parnurzeal/gorequest)
- [goquery](https://github.com/PuerkitoBio/goquery)
- [chromedp](https://github.com/chromedp/chromedp)
- [markdown](https://github.com/gomarkdown/markdown)
- [webp](https://github.com/chai2010/webp)
- [cron](https://github.com/robfig/cron)
- [open-golang](https://github.com/skratchdot/open-golang)
- [go-qrcode](https://github.com/skip2/go-qrcode)

## 访问管理后台
如果你从 GitHub 上克隆下载的代码，自行编译运行的话，需要先编译后台的管理代码，后台管理代码在 https://github.com/fesiong/anqicms-admin 。
你也可以从 后台管理代码的release中，下载最新的release，将system.zip 解压到项目根目录下的system文件夹。

后台地址默认为 http://127.0.0.1:8001/system

如果你不是通过安装初始化的话，可能没有设置管理员账号，如果没有设置管理员账号，默认的管理员账号密码分别是：

账号：admin

密码：123456

## 示例网站 & 开发文档
[示例网站 - https://www.anqicms.com/](https://www.anqicms.com/manual)


## 👥问题反馈    
遇到问题, 请在Github上开issue。  
也可以加我的微信：websafety

扫码加入golang开发学习群

![扫码入群讨论](https://www.anqicms.com/uploads/202211/09/1a55bfcde55aa2d6.webp)

## License
AnqiCMS 最终用户授权协议

Copyright (c) 2019-NOW  Fesion <tpyzlxy@gmail.com>
