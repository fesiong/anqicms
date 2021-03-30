# GoBlog 基于irisweb的golang编写的简洁版个人博客系统

GoBlog是一个开源的个人博客系统，界面优雅，小巧、执行速度飞快，并且对seo友好，可以满足日常博客需求。你完全可以用它来搭建自己的博客。它的使用很简单，部署非常方便。  
  
GoBlog是一款由golang编写的博客，它使用了golang非常流行的网页框架irisweb+gorm，pc和移动端自适应，页面模板使用类似blade模板引擎语法，上手非常容易。  
  
GoBlog同时支持小程序接口，小程序端使用Taro跨平台框架开发，将同时支持微信小程序、百度智能小程序、QQ小程序、支付宝小程序，字节跳动小程序等。

## GoBlog 分支版本说明

- master为最新开发版代码
- simple 为仅包含基础功能的博客代码
- blog 为具有完整后台的博客代码
- enterprise 为博客基础上加入了企业站功能的代码

## GoBlog 开发计划表

### simple 基础功能 (已发布)

- [x] 博客底层功能
- [x] 发布/修改文章
- [x] 创建分类
- [x] 文章展示
- [x] 图片上传
- [x] 初始化博客
- [x] 页面tdk设置
- [x] 管理员登录/权限控制
- [x] pc端和移动端自适应适配

### blog 博客完善 (已发布)

- [x] 增加管理后台
- [x] 自动提取和设置缩略图
- [x] sitemap自动生成
- [x] robots后台配置
- [x] 搜索引擎主动推送
- [x] 友情链接后台管理
- [x] 增加文章评论和评论管理
- [x] 增加闭站功能
- [x] 自动过滤外链
- [x] 自定义导航配置功能
- [x] 优雅的启动和重启博客

### enterprise 企业站适配 (开发中)

- [x] 增加多模板支持功能
- [x] 增加产品模块和管理
- [x] 后台动态设置前端基础信息
- [x] 自定义url伪静态规则
- [x] 增加自动添加锚文本功能
- [x] 增加留言功能
- [x] 增加关键词管理
- [x] 增加内容素材片段管理
- [x] 增加留言邮件提醒功能
- [ ] 增加内容采集功能
- [x] 增加后台上传各种验证文件功能
- [ ] 增加后台直接修改模板功能
- [x] 增加蜘蛛、流量统计和配置功能
- [ ] 增加自定义文章、产品字段功能
- [x] 增加移动端模板功能
- [x] 增加计划任务功能

### 其他计划

- [ ] 适配微信小程序
- [ ] 适配百度小程序
- [ ] 适配头条小程序
- [ ] 适配支付宝小程序
- [ ] 适配QQ小程序

## GoBlog 的安装
### GoBlog依赖的软件
| 软件 | 版本|  
|:---------|:-------:|
| golang  |  1.13 (及以上) |
| mysql  |  5.6.35 (及以上) |

### 克隆代码
将`GoBlog`的代码克隆到本地任意目录，并进入该目录操作  
### 安装依赖环境
由于众所周知的原因，我们需要设置代理，在终端执行这个代码：
```bash
go env -w GOPROXY=https://goproxy.cn,direct
```
接着执行下面的代码，编译代码是编译成可执行文件，测试运行可以一边测试一边修改。
```bash
go mod tidy
go mod vendor
# 这是编译代码
go build -o GoBlog app/main.go
# 这是测试运行代码
go run app/main.go
```
至此便可以运行网站了
### 运行GoBlog
启动GoBlog
```bash
# 这是执行编译后的可执行文件
./GoBlog
# 这是测试运行代码
go run app/main.go
```
在浏览器访问： http://127.0.0.1:8001 。初次访问，需要初始化GoBlog，在初始化界面，输入mysql信息，设置管理员账号、密码。完成后，就可以开始编写博客了。
### 服务端部署
本地测试没有问题后，就可以将代码打包到服务器上编译了。也可以在本地根据服务器系统编译好可执行文件，将可执行文件放到服务器上。一般上，还需要配置nginx代理，来使用80端口或https端口。  
nginx代理代码如下：
```bash
    location @GoBlog {
        proxy_pass http://127.0.0.1:8001;
        proxy_set_header   Host             $host;
        proxy_set_header   X-Real-IP        $remote_addr;
        proxy_set_header   X-Forwarded-For  $proxy_add_x_forwarded_for;
    }
    location / {
       try_files $uri $uri/index.html @GoBlog;
    }
```

### 访问管理后台
管理后台在blog分支和master分支上提供，simple分支没有管理后台。

后台地址默认为 http://127.0.0.1:8001/manage

你可以在登录后台后修改后台地址。如果修改后台地址后，需要重启才能生效。

如果你不是通过安装初始化博客的话，可能没有设置管理员账号，如果没有设置管理员账号，默认的管理员账号密码分别是：

账号：admin

密码：123456

## 示例网站 & 开发文档 & golang实战学习教程
[示例网站 - https://www.kandaoni.com/](https://www.kandaoni.com/)  

[实战学习教程](https://www.kandaoni.com/category/1)  


## 👥问题反馈    
遇到问题, 请在Github上开issue。  
也可以加我的微信：no_reg

扫码加入golang开发学习群

![扫码入群讨论](https://www.kandaoni.com/uploads/20213/3/thumb_1525154eb779f3c7.png)

## License
The MIT License (MIT)

Copyright (c) 2019-NOW  Fesion <tpyzlxy@163.com>
