# GoBlog 基于irisweb的golang编写的简洁版个人博客系统

goblog是一个开源的个人博客系统，界面优雅，小巧、执行速度飞快，并且对seo友好，可以满足日常博客需求。你完全可以用它来搭建自己的博客。它的使用很简单，部署非常方便。  
  
goblog是一款由golang编写的博客，它使用了golang非常流行的网页框架irisweb+gorm，pc和移动端自适应，页面模板使用类似blade模板引擎语法，上手非常容易。  
  
goblog同时支持小程序接口，小程序端使用Taro跨平台框架开发，将同时支持微信小程序、百度智能小程序、QQ小程序、支付宝小程序，字节跳动小程序等。

## goblog 的安装
### goblog依赖的软件
| 软件 | 版本|  
|:---------|:-------:|
| golang  |  1.9 (及以上) |
| mysql  |  5.6.35 (及以上) |

### 克隆代码
将`goblog`的代码克隆到本地任意目录，并进入该目录操作  
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
go build -o goblog app/main.go
# 这是测试运行代码
go run app/main.go
```
至此便可以运行网站了
### 运行goblog
启动goblog
```bash
# 这是执行编译后的可执行文件
./goblog
# 这是测试运行代码
go run app/main.go
```
在浏览器访问： http://127.0.0.1:8001 。初次访问，需要初始化goblog，在初始化界面，输入mysql信息，设置管理员账号、密码。完成后，就可以开始编写博客了。
### 服务端部署
本地测试没有问题后，就可以将代码打包到服务器上编译了。也可以在本地根据服务器系统编译好可执行文件，将可执行文件放到服务器上。一般上，还需要配置nginx代理，来使用80端口或https端口。  
nginx代理代码如下：
```bash
    location @goblog {
        proxy_pass http://127.0.0.1:8001;
        proxy_set_header   Host             $host;
        proxy_set_header   X-Real-IP        $remote_addr;
        proxy_set_header   X-Forwarded-For  $proxy_add_x_forwarded_for;
    }
    location / {
       try_files $uri $uri/index.html @go;
    }
```
## 👥问题反馈    
遇到问题, 请在Github上开issue。
## License
The MIT License (MIT)

Copyright (c) 2019-NOW  Fesion <tpyzlxy@163.com>