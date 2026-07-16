---
name: api-dev
description: AnQiCMS API 开发技能：全部 REST API 端点、请求/响应格式、认证流程、关键工作流。适用于前端对接和二次开发
category: Development
version: 1.0
tags: [anqicms, api, rest, frontend]
---

# AnQiCMS API 开发技能

## 基础规范
- **路径前缀**：`/api`
- **格式**：JSON（文件上传除外）
- **认证**：Header `Token: <token>`（不是 `Authorization`），`code === 1001` 时跳到登录页
- **分页**：`page`、`limit`（默认10）、`order`（如 `id desc`）
- **响应**：`{ code: 0, msg: "成功", data: T, total?: number }`，始终检查 `code === 0`

## 公共 API（无需认证）

### 内容
| 端点 | 说明 |
|---|---|
| `GET /api/archive/list?moduleId=&categoryId=&limit=&flag=&q=` | 文档列表，`moduleId=1`文章 `2`产品 |
| `GET /api/archive/detail?id=` | 文档详情 |
| `GET /api/archive/params?id=` | 文档自定义参数 |
| `GET /api/archive/prev?id=` / `GET /api/archive/next?id=` | 上下篇 |
| `GET /api/category/list?type=list\|tree` | 分类列表 |
| `GET /api/category/detail?id=` | 分类详情 |
| `GET /api/tag/list` | 标签列表 |
| `GET /api/tag/detail?id=` | 标签详情 |
| `GET /api/tag/data/list?tagId=` | 标签下的文档 |
| `GET /api/page/list` | 单页面列表 |
| `GET /api/page/detail?id=` | 单页面详情 |

### 站点信息
| 端点 | 说明 |
|---|---|
| `GET /api/setting/system` | 系统设置（站点名称、SEO等） |
| `GET /api/setting/index` | 首页配置 |
| `GET /api/setting/contact` | 联系方式 |
| `GET /api/nav/list` | 导航菜单 |
| `GET /api/banner/list?type=` | Banner/幻灯片 |
| `GET /api/languages` | 多语言列表 |
| `GET /api/friendlink/list` | 友情链接 |
| `GET /api/comment/list?archive_id=` | 评论列表 |

### 用户认证
| 端点 | 说明 |
|---|---|
| `POST /api/login` | 登录，返回 token |
| `POST /api/register` | 注册 |
| `GET /api/captcha` | 验证码 |
| `POST /api/verify/email` | 邮箱验证 |

## 需认证 API（需要 Token Header）

### 用户中心
| 端点 | 说明 |
|---|---|
| `GET /api/user/detail` | 用户信息 |
| `POST /api/user/update` | 更新资料 |
| `POST /api/user/avatar` | 上传头像 |
| `GET /api/favorite/list` | 收藏列表 |
| `POST /api/favorite/add` / `POST /api/favorite/delete` | 添加/删除收藏 |
| `GET /api/favorite/check?archive_id=` | 检查是否已收藏 |

### 购物车
| 端点 | 说明 |
|---|---|
| `GET /api/cart/list` | 购物车列表 |
| `POST /api/cart/add` | 添加（`archive_id`, `quantity`, `sku_id`） |
| `POST /api/cart/update` | 更新数量（`id`, `quantity`） |
| `POST /api/cart/remove` | 删除（`id`） |

### 订单
| 端点 | 说明 |
|---|---|
| `POST /api/orders/checkout` | 预结算 |
| `POST /api/order/create` | 创建订单 |
| `GET /api/orders` | 订单列表 |
| `GET /api/order/detail?id=` | 订单详情 |
| `POST /api/order/cancel` | 取消订单 |
| `POST /api/order/payment` | 支付 |
| `POST /api/order/finished` | 确认收货 |
| `POST /api/order/refund` | 申请退款 |
| `GET /api/order/addresses` | 地址列表 |
| `POST /api/order/address/save` | 保存地址 |

### 交互
| 端点 | 说明 |
|---|---|
| `POST /api/comment/publish` | 发布评论 |
| `POST /api/comment/praise` | 点赞评论 |
| `POST /api/guestbook.html` | 提交留言 |
| `POST /api/attachment/upload` | 上传文件（multipart） |

### 分销（零售商）
| 端点 | 说明 |
|---|---|
| `GET /api/retailer/info` | 分销信息 |
| `GET /api/retailer/statistics` | 分销统计 |
| `GET /api/retailer/members` | 团队成员 |
| `GET /api/retailer/orders` | 分销订单 |
| `POST /api/retailer/withdraw` | 提现 |

## 关键工作流

### 全局数据（页面布局用）
```
GET /api/setting/system  → 站点名称、SEO
GET /api/nav/list        → 导航
GET /api/banner/list     → 幻灯片
GET /api/setting/contact → 联系方式
GET /api/languages       → 多语言
```

### 下单流程
```
POST /api/cart/add          → 加购物车
POST /api/orders/checkout   → 预结算
POST /api/order/create      → 创建订单
POST /api/order/payment     → 支付
```

### 错误处理
```typescript
try {
  const res = await fetch("/api/xxx");
  const json = await res.json();
  if (json.code === 0) {
    // 成功，使用 json.data
  } else if (json.code === 1001) {
    // 登录过期，跳转 /login
  } else {
    // 显示 json.msg
  }
} catch (e) {
  // 网络错误处理
}
```
