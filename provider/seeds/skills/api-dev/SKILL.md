---
name: api-dev
description: AnQiCMS 前端公共 API（/api）开发技能：真实路由清单、请求/响应格式、鉴权与分页约定、关键工作流。适用于前端对接和二次开发
category: Development
version: 1.1
tags: [anqicms, api, rest, frontend]
---

# AnQiCMS 前端 API 开发技能

本技能描述**面向前台访客/前端 JS 的公共 API**（前缀 `/api`，由 Iris 的 `app.Party("/api", ParseUserToken)` 注册，见 `route/base.go`）。它与后台管理 API（前缀 `/system/api`，走 `Admin` 鉴权）是两套不同的接口，不要混用。

## 基础规范
- **路径前缀**：`/api`
- **格式**：JSON（文件上传为 multipart）
- **鉴权 Header**：`token: <jwt>`（也兼容 `Authorization: Bearer <jwt>` 与 cookie `token`）。**不是** `Admin`（那是后台）。
  - token 采用**滑动轮换**：需要续期时，新 token 放在**响应头 `update-token`** 里，前端应读取并替换本地 token。
  - 登录成功后 token 在 `data.token`。
- **公共内容 API 需站点开启**：带 `CheckApiOpen` 的只读内容接口（archive/category/tag/page/setting 等）只有在后台开启"API 访问"后才可用，否则返回未开放。
- **响应体**：`{ code, msg, data, total? }`，`total` 仅列表接口返回。**始终以 `code` 判定成败，HTTP 200 不代表成功。**

### 响应码（定义于 config/constant.go）
| code | 含义 | 前端处理 |
|---|---|---|
| `0` | 成功（StatusOK） | 使用 `data` |
| `-1` | 失败（StatusFailed） | 显示 `msg` |
| `1001` | 未登录/token 失效（StatusNoLogin） | 跳转登录页 |
| `1002` | 无权限（StatusNoAccess） | 提示权限不足 |

### 分页（两套词表，按接口区分）
- **内容列表**（`archive/list`、`tag/list`、`tag/data/list`、`comment/list`）：用 `page` + `limit`。`limit` 可为 `"n"` 或 `"offset,n"`，上限受后台 `Content.MaxLimit` 约束。`page/list` 只读 `limit`。
- **用户/订单/分销列表**（`favorite/list`、`orders`、`retailer/*`）：用 `current` + `pageSize`（默认 20）。

## 公共只读 API（无需登录，需开启 API 访问）

### 内容
| 端点 | 说明 |
|---|---|
| `GET /api/archive/list?moduleId=&categoryId=&page=&limit=&flag=&q=` | 文档列表，`moduleId=1` 文章、`2` 产品 |
| `GET /api/archive/detail?id=` | 文档详情 |
| `GET /api/archive/params?id=` | 文档自定义参数 |
| `GET /api/archive/filters` | 筛选项 |
| `GET /api/archive/prev?id=` / `GET /api/archive/next?id=` | 上一篇 / 下一篇 |
| `GET /api/category/list?type=list\|tree` / `GET /api/category/detail?id=` | 分类列表 / 详情 |
| `GET /api/module/list` / `GET /api/module/detail?id=` | 内容模型列表 / 详情 |
| `GET /api/tag/list` / `GET /api/tag/detail?id=` | 标签列表 / 详情 |
| `GET /api/tag/data/list?tagId=` | 标签下的文档 |
| `GET /api/page/list` / `GET /api/page/detail?id=` | 单页面列表 / 详情 |
| `GET /api/place/list` / `GET /api/place/detail?id=` | 推荐位列表 / 详情 |
| `GET /api/comment/list?archive_id=` | 评论列表 |

### 站点信息
| 端点 | 说明 |
|---|---|
| `GET /api/setting/system` | 系统设置（站点名称、SEO 等） |
| `GET /api/setting/index` | 首页 TDK 配置 |
| `GET /api/setting/contact` | 联系方式 |
| `GET /api/setting/diy` | 自定义字段 |
| `GET /api/nav/list` | 导航菜单 |
| `GET /api/banner/list?type=` | Banner / 幻灯片 |
| `GET /api/languages` | 多语言列表 |
| `GET /api/friendlink/list` | 友情链接（只读列表公开） |
| `GET /api/guestbook/fields` | 留言板表单字段 |
| `GET /api/metadata` | 站点元数据 |

## 认证与用户（无需登录的部分）
| 端点 | 说明 |
|---|---|
| `POST /api/login` | 登录，返回 `data.token` |
| `POST /api/register` | 注册 |
| `GET /api/captcha` | 图形验证码 |
| `GET /api/verify/email` / `POST /api/verify/email` | 校验 / 发送邮箱验证 |
| `POST /api/password/reset` | 重置密码 |
| `GET /api/user/groups` / `GET /api/user/group/detail` | 用户组列表 / 详情 |

## 需登录 API（`token` Header + UserAuth）
### 用户中心
| 端点 | 说明 |
|---|---|
| `GET /api/user/detail` | 用户信息 |
| `POST /api/user/detail` | 更新资料 |
| `POST /api/user/avatar` | 上传头像 |
| `POST /api/user/password` | 修改密码 |
| `GET /api/favorite/list` | 收藏列表（`current`/`pageSize`） |
| `POST /api/favorite/check` | 检查是否已收藏 |
| `POST /api/favorite/add` / `POST /api/favorite/delete` | 添加 / 删除收藏 |

### 订单
| 端点 | 说明 |
|---|---|
| `GET /api/orders` | 我的订单列表（`current`/`pageSize`，需登录） |
| `POST /api/order/create` | 创建订单（公开） |
| `GET /api/order/detail?id=` | 订单详情（公开） |
| `POST /api/order/payment` | 发起支付（公开） |
| `POST /api/order/cancel` | 取消订单（需登录） |
| `POST /api/order/refund` | 申请退款（需登录） |
| `POST /api/order/finish` | 确认收货（需登录，注意是 `finish` 不是 `finished`） |
| `GET /api/order/addresses` | 地址列表（需登录） |
| `GET /api/order/address` / `POST /api/order/address` | 取单个 / 保存地址（需登录） |
| `GET /api/payment/check` / `GET /api/archive/order/check` | 支付结果 / 文档购买校验（公开） |
| `POST /api/archive/password/check` | 校验加密文档密码（公开） |
| `POST /api/archive/publish` | 前台投稿发布文档（需登录） |

### 分销（零售商）
| 端点 | 说明 |
|---|---|
| `GET /api/retailer/info` | 分销信息（公开） |
| `GET /api/retailer/statistics` | 分销统计（需登录） |
| `POST /api/retailer/update` | 更新分销资料（需登录） |
| `GET /api/retailer/orders` | 分销订单（需登录） |
| `GET /api/retailer/members` | 团队成员（需登录） |
| `GET /api/retailer/commissions` | 佣金明细（需登录） |
| `GET /api/retailer/withdraw` / `POST /api/retailer/withdraw` | 提现记录 / 申请提现（需登录） |

### 交互（需开启 API 访问）
| 端点 | 说明 |
|---|---|
| `POST /api/comment/publish` | 发布评论 |
| `POST /api/comment/praise` | 点赞评论（**另需登录**） |
| `POST /api/guestbook.html` | 提交留言 |
| `POST /api/subscription` | 订阅（邮件/推送） |
| `POST /api/attachment/upload` | 上传文件（multipart） |

## 其它入口
| 端点 | 说明 |
|---|---|
| `POST /api/graphql` / `GET /api/playground` | GraphQL v2 及调试台 |
| `ANY /api/mcp` | MCP 协议端点，**Bearer token** 鉴权（`McpTokenAuth`） |
| `GET /api/log` | 前端日志上报/统计 |
| `GET /api/wechat/auth`、`GET\|POST /api/wechat`、`POST /api/weapp/qrcode` | 微信公众号 / 小程序 |
| `POST /api/friendlink/create\|delete`、`GET\|POST /api/friendlink/check` | 友链导入，需 `VerifyApiLinkToken`（专用 link token，非用户 token） |
| 支付回调 | `POST /notify/wechat/pay`、`/notify/alipay/pay`、`/notify/paypal/pay`、`/notify/weapp/msg`；`GET /return/paypal/pay\|cancel` |

> **注意**：本站**没有购物车（cart）接口**，下单直接走 `POST /api/order/create`。历史文档里的 `/api/cart/*`、`/api/orders/checkout` 均不存在。

## 关键工作流

### 全局数据（页面布局用）
```
GET /api/setting/system  → 站点名称、SEO
GET /api/nav/list        → 导航
GET /api/banner/list     → 幻灯片
GET /api/setting/contact → 联系方式
GET /api/languages       → 多语言
```

### 下单流程（无购物车）
```
POST /api/order/create   → 创建订单（直接下单）
POST /api/order/payment  → 发起支付
GET  /api/payment/check  → 轮询支付结果
```

### 错误处理
```typescript
try {
  const res = await fetch("/api/xxx", { headers: { token: localToken } });
  // token 轮换：优先用响应头里的新 token
  const rotated = res.headers.get("update-token");
  if (rotated) localToken = rotated;

  const json = await res.json();
  if (json.code === 0) {
    // 成功，使用 json.data（列表另有 json.total）
  } else if (json.code === 1001) {
    // 未登录/过期，跳转 /login
  } else {
    // 显示 json.msg（-1 失败、1002 无权限）
  }
} catch (e) {
  // 网络错误处理
}
```

> 完整的端点级参数与后台管理接口，用 `api` 意图（action=list/schema，默认关闭需显式开启）在线查询，或加载 `anqicms-dev` 技能。
