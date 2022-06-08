# AnqiCMS 基于irisweb的golang编写的安企内容管理系统

安企内容管理系统(AnqiCMS)，是一款使用 GoLang 开发的企业站内容管理系统，它部署简单，软件安全，界面优雅，小巧，执行速度飞快，使用 AnqiCMS 搭建的网站可以防止众多安全问题发生。AnqiCMS 的设计对SEO友好，并且内置了大量企业站常用功能，对网站优化有很好的帮助提升，对企业管理网站一定程度上提供了办事效率，提高企业的竞争力。

AnqiCMS 除了适合做企业站，也适合做营销型网站、企业官网、商品展示站点、政府网站、门户网站、个人博客等等各种类型的网站。AnqiCMS 是什么，AnqiCMS 是一个可以自由使用并开放源码的内容管理系统，你可以拿 AnqiCMS 来搭建各种不违法的网站。

AnqiCMS 支持 Django 模板引擎语法，该语法类似 blade 语法，可以非常容易上手模板制作。网站模式支持 自适应、代码适配、PC+mobile独立站点 模式，根据不用需求，可以选择适合自己的搭配方式来建站。

我们的追求：让天下都是安全的网站。

我们一直朝着网站安全的方向前进，让 AnqiCMS 为你的网站安全护航。

欢迎您使用 AnqiCMS。

## AnqiCMS 分支版本说明

- master为最新开发版代码
- simple 为仅包含基础功能的博客代码
- blog 为具有完整后台的博客代码
- enterprise 为安企内容管理系统(AnqiCMS)的代码

## AnqiCMS 开发计划表

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

### enterprise 安企内容管理系统(AnqiCMS) (已发布)

- [x] 增加多模板支持功能
- [x] 增加产品模块和管理
- [x] 后台动态设置前端基础信息
- [x] 自定义url伪静态规则
- [x] 增加自动添加锚文本功能
- [x] 增加留言功能
- [x] 增加关键词管理
- [x] 增加内容素材片段管理
- [x] 增加留言邮件提醒功能
- [x] 增加后台上传各种验证文件功能
- [x] 增加蜘蛛、流量统计和配置功能
- [x] 增加自定义文章、产品字段功能
- [x] 增加移动端模板功能
- [x] 增加计划任务功能
- [x] 增加模板标签功能[查看标签使用手册](https://www.kandaoni.com/category/10)
- [x] 增加自定义分类模板、文章模板、产品模板、页面模板功能
- [x] 增加内容采集功能
- [x] 增加内容伪原创功能 ？该功能还需要调试
- [ ] 增加后台直接修改模板功能
- [ ] 增加标签功能

### 其他计划

- [ ] 适配微信小程序
- [ ] 适配百度小程序
- [ ] 适配头条小程序
- [ ] 适配支付宝小程序
- [ ] 适配QQ小程序

### 模板标签
[查看模板标签说明](https://www.kandaoni.com/category/10)

## AnqiCMS 的安装
### AnqiCMS依赖的软件
| 软件 | 版本|  
|:---------|:-------:|
| golang  |  1.13 (及以上) |
| mysql  |  5.6.35 (及以上) |

### 克隆代码
将`AnqiCMS`的代码克隆到本地任意目录，并进入该目录操作  
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
go build -o anqicms main/main.go
# 这是测试运行代码
go run main/main.go
```
至此便可以运行网站了
### 运行AnqiCMS
启动AnqiCMS
```bash
# 这是执行编译后的可执行文件
./anqicms
# 这是测试运行代码
go run main/main.go
```
在浏览器访问： http://127.0.0.1:8001 。初次访问，需要初始化AnqiCMS，在初始化界面，输入mysql信息，设置管理员账号、密码。完成后，就可以开始编写博客了。
### 服务端部署

下载最新的发行版：[下载发行版](https://github.com/fesiong/goblog/releases)

根据你的服务器选择，linux用户，请下载anqicms-linux.zip，windows用户，请下载anqicms-windows.zip。

将压缩包上传的网站根目录。

#### nginx 配置

一般上，还需要配置nginx代理，来使用80端口或https端口。  
nginx代理代码如下：
```bash
    location @AnqiCMS {
        proxy_pass http://127.0.0.1:8001;
        proxy_set_header   Host             $host;
        proxy_set_header   X-Real-IP        $remote_addr;
        proxy_set_header   X-Forwarded-For  $proxy_add_x_forwarded_for;
    }
    error_page 404 =200  @AnqiCMS;
    location / {
       try_files $uri $uri/index.html @AnqiCMS;
    }
```

完整的nginx配置：
```conf
server
{
    listen       80;
    server_name dev.anqicms.com m.anqicms.com;
    root /data/wwwroot/irisweb/public;

    location @AnqiCMS {
        proxy_pass http://127.0.0.1:8001;
        proxy_set_header   Host             $host;
        proxy_set_header   X-Real-IP        $remote_addr;
        proxy_set_header   X-Forwarded-For  $proxy_add_x_forwarded_for;
    }
    error_page 404 =200  @AnqiCMS;
    location / {
       try_files $uri $uri/index.html @AnqiCMS;
    }
    access_log off;
}
```

#### 宝塔下Apache环境部署

- 调整运行目录，运行目录设置为：/public
- 设置反向代理，目标URL：http://127.0.0.1:8001
- 添加计划任务脚本，执行周期，每分钟执行一次，内容为：
```shell script
#!/bin/bash
### check 502
# author fesion
# the bin name is anqicms
BINNAME=anqicms
# 设置anqicms目标目录
BINPATH=/你存放anqicms的目录

# check the pid if exists
exists=`ps -ef | grep '\<anqicms\>' |grep -v grep |wc -l`
echo "$(date +'%Y%m%d %H:%M:%S') $BINNAME PID check: $exists" >> $BINPATH/check.log
echo "PID $BINNAME check: $exists"
if [ $exists -eq 0 ]; then
    echo "$BINNAME NOT running"
    cd $BINPATH && nohup $BINPATH/$BINNAME >> $BINPATH/running.log 2>&1 &
fi
```
添加计划任务后，点执行。

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
AnqiCMS 最终用户授权协议

Copyright (c) 2019-NOW  Fesion <tpyzlxy@163.com>
