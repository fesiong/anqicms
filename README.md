# GoBlog   

goblog 是一个开源的个人博客系统，界面优雅，小巧迅速，并且原生对SEO很友好，满足日常博客需求，你完全可以用它来搭建自己的博客。       

goblog的技术架构是前后端分离的, 前端使用**react**、**antd-mobile**、**node.js**、**next.js**等技术来开发, 后端使用**go**、**gin**、**gorm**等技术来开发。goblog的技术选型，大胆抛弃传统的php+html模板技术, 我们大胆的使用**next.js**来做**前后端同构渲染**，pc与移动端自适应。

> 更新预告：即将择时推出小程序端，使用的是Taro框架，将同时支持微信、百度、支付宝、字节跳动小程序


## 🚀 安装

### 依赖的软件

| 软件 | 版本|  
|:---------|:-------:|
| node.js     |  8.4.0 (及以上) |
| golang  |  1.9 (及以上) |
| mysql  |  5.6.35 (及以上) |

### 克隆代码
将`goblog`的代码克隆到gopath的src/目录下，即`your/gopath/src/goblog`

### 前端依赖的模块
进入`goblog/website`目录，输入命令

```
npm install
```

如果安装失败，或速度慢，可尝试阿里的镜像

```
npm install --registry=https://registry.npm.taobao.org
```

### 后端依赖的库

goblog使用dep来管理依赖的包，请先安装dep, 执行以下命令即完成安装

```
go get -u github.com/golang/dep/cmd/dep
```

然后，在 **goblog** 项目目录下运行以下命令来安装依赖

```
dep ensure
```

## ⚙️ 配置
### hosts   
127.0.0.1 dev.goblog.com  

### nginx 
1. 将`goblog/nginx/dev.goblog.com.example.conf`文件改名为`dev.goblog.com.conf`，然后拷贝到nginx的虚拟主机目录下
2. 将`goblog/nginx/server.key`和`goblog/nginx/server.crt`拷贝到某个目录下
3. 打开nginx的虚拟主机目录下的`dev.goblog.com.conf`文件，然后修改访问日志和错误日志的路径，即修改access\_log和error\_log。

请参考如下配置中`请修改`标记的地方:

```
server {
    listen 80;
    server_name dev.goblog.com;

    access_log /path/logs/goblog.access.log; #请修改
    error_log /path/logs/goblog.error.log;   #请修改

    ...
}
```

4. 实际线上环境配置的时候，建议使用https， 可以使用letsencrypt免费证书。

### 前端配置
将`goblog/website/utils/config.example.js`文件重命名为`config.js`

### 后端配置
将`goblog/config.example.json`文件重命名为`config.json`，然后修改以下配置:  

1. 修改mysql连接地址及端口
2. 修改mysql的用户名及密码
5. 将`goblog/sql/goblog.sql`导入到你自己的数据库中

## 🚕 运行
### 运行前端项目
进入`goblog/website`目录，然后运行

```
npm run dev
```

### 运行后端项目
进入`goblog`目录，然后运行

```
go run main.go
```

### 访问
首页: http://dev.goblog.com    
管理员登录: http://dev.goblog.com/sign/in  
### 创建管理员账号
访问http://dev.goblog.com/sign/up ，第一个注册的账号即为管理员账号，请谨记账号密码，如果忘记了，到数据删除掉所有用户，一个不剩的时候，再注册一个就可以。

## 👥问题反馈    
遇到问题, 请在Github上开issue。

## License
The MIT License (MIT)

Copyright (c) 2019-NOW  Fesion <tpyzlxy@gmail.com>
