/**
 * @name umi 的路由配置
 * @description 只支持 path,component,routes,redirect,wrappers,name,icon 的配置
 * @param path  path 只支持两种占位符配置，第一种是动态参数 :id 的形式，第二种是 * 通配符，通配符只能出现路由字符串的最后。
 * @param component 配置 location 和 path 匹配后用于渲染的 React 组件路径。可以是绝对路径，也可以是相对路径，如果是相对路径，会从 src/pages 开始找起。
 * @param routes 配置子路由，通常在需要为多个路径增加 layout 组件时使用。
 * @param redirect 配置路由跳转
 * @param wrappers 配置路由组件的包装组件，通过包装组件可以为当前的路由组件组合进更多的功能。 比如，可以用于路由级别的权限校验
 * @param name 配置路由的标题，默认读取国际化文件 menu.ts 中 menu.xxxx 的值，如配置 name 为 login，则读取 menu.ts 中 menu.login 的取值作为标题
 * @param icon 配置路由的图标，取值参考 https://ant.design/components/icon-cn， 注意去除风格后缀和大小写，如想要配置图标为 <StepBackwardOutlined /> 则取值应为 stepBackward 或 StepBackward，如想要配置图标为 <UserOutlined /> 则取值应为 user 或者 User
 * @doc https://umijs.org/docs/guides/routes
 */
export default [
  {
    path: '/login',
    layout: false,
    routes: [
      {
        path: '/login',
        name: 'login',
        component: './login',
      },
    ],
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    icon: 'home',
    hideInTop: true,
    component: './dashboard/index',
  },
  {
    path: '/setting',
    name: 'setting',
    icon: 'setting',
    access: 'canAdmin',
    routes: [
      {
        path: '/setting/system',
        name: 'system',
        component: './setting/system',
      },
      {
        path: '/setting/contact',
        name: 'contact',
        component: './setting/contact',
      },
      {
        path: '/setting/content',
        name: 'content',
        component: './setting/content',
      },
      {
        path: '/setting/safe',
        name: 'safe',
        component: './setting/safe',
      },
      {
        path: '/setting/sensitive',
        name: 'sensitive',
        component: './setting/sensitive',
      },
      {
        path: '/setting/tdk',
        name: 'tdk',
        component: './setting/index',
      },
      {
        path: '/setting/banner',
        name: 'banner',
        component: './setting/banner',
      },
      {
        path: '/setting/nav',
        name: 'nav',
        component: './setting/nav',
      },
      {
        path: '/setting/ai',
        name: 'ai',
        component: './setting/ai',
      },
      {
        path: '/setting/currency',
        name: 'currency',
        component: './setting/currency',
      },
      {
        path: '/setting/pay',
        name: 'pay',
        component: '@/pages/plugin/pay/index',
      },
    ],
  },
  {
    path: '/order',
    name: 'order',
    icon: 'accountBook',
    routes: [
      {
        path: '/order/index',
        name: 'order',
        component: '@/pages/plugin/order/index',
      },
      {
        path: '/order/recall',
        name: 'recall',
        component: '@/pages/plugin/order/recall',
      },
      {
        path: '/order/waybill',
        name: 'waybill',
        component: '@/pages/plugin/order/waybill',
      },
      {
        path: '/order/communication',
        name: 'communication',
        component: '@/pages/plugin/order/communication',
      },
      {
        path: '/order/service',
        name: 'service',
        component: '@/pages/plugin/order/service',
      },
      {
        path: '/order/shipping',
        name: 'shipping',
        component: '@/pages/plugin/shipping/index',
      },
    ],
  },
  {
    path: '/product',
    name: 'product',
    icon: 'product',
    routes: [
      {
        path: '/product/index',
        name: 'product',
        component: '@/pages/content/product/index',
      },
      {
        hideInMenu: true,
        path: '/product/detail',
        component: '@/pages/content/product/detail',
      },
      {
        path: '/product/category',
        name: 'category',
        component: '@/pages/content/category/product/index',
      },
      {
        hideInMenu: true,
        path: '/product/category/detail',
        component: '@/pages/content/category/product/detail',
      },
      // {
      //   path: '/product/inventory',
      //   name: 'inventory',
      //   component: '@/pages/content/product/inventory',
      // },
      {
        path: '/product/review',
        name: 'review',
        component: '@/pages/content/product/review',
      },
    ],
  },
  {
    path: '/customer',
    name: 'customer',
    icon: 'team',
    routes: [
      {
        path: '/customer/index',
        name: 'customer',
        component: '@/pages/plugin/user/index',
      },
      {
        path: '/customer/group',
        name: 'group',
        component: '@/pages/plugin/group/index',
      },
      {
        path: '/customer/guestbook',
        name: 'guestbook',
        component: '@/pages/plugin/guestbook/index',
      },
      {
        path: '/customer/wishlist',
        name: 'wishlist',
        component: '@/pages/customer/wishlist',
      },
    ],
  },
  {
    path: '/promotion',
    name: 'promotion',
    icon: 'fund',
    routes: [
      // {
      //   path: '/promotion/index',
      //   name: 'promotion',
      //   component: '@/pages/promotion/index',
      // },
      {
        path: '/promotion/automation',
        name: 'automation',
        component: '@/pages/plugin/sendmail/index',
      },
      {
        path: '/promotion/subscription',
        name: 'subscription',
        component: '@/pages/promotion/subscription',
      },
      {
        path: '/promotion/googlemerchant',
        name: 'googlemerchant',
        component: '@/pages/promotion/googlemerchant/index',
      },
      {
        path: '/promotion/googlemerchant/callback',
        component: '@/pages/promotion/googlemerchant/callback',
      },
    ],
  },
  {
    path: '/sale',
    name: 'sale',
    icon: 'percentage',
    routes: [
      {
        path: '/sale/index',
        name: 'coupon',
        component: '@/pages/sale/coupon',
      },
      {
        path: '/sale/sales',
        name: 'sales',
        component: '@/pages/sale/sales',
      },
      {
        path: '/sale/activity',
        name: 'activity',
        component: '@/pages/sale/activity',
      },
      // {
      //   path: '/sale/package',
      //   name: 'package',
      //   component: '@/pages/sale/package',
      // },
      // {
      //   path: '/sale/freeshipping',
      //   name: 'freeshipping',
      //   component: '@/pages/sale/freeshipping',
      // },
      // {
      //   path: '/sale/buygifts',
      //   name: 'buygifts',
      //   component: '@/pages/sale/buygifts',
      // },
    ],
  },
  {
    path: '/archive',
    name: 'archive',
    icon: 'profile',
    access: 'canAdmin',
    routes: [
      {
        path: '/archive/list',
        name: 'list',
        component: '@/pages/content/archive/index',
      },
      {
        path: '/archive/recycle',
        hideInMenu: true,
        name: 'recycle',
        component: '@/pages/content/archive/recycle',
      },
      {
        hideInMenu: true,
        path: '/archive/detail',
        component: '@/pages/content/archive/detail',
      },
      {
        path: '/archive/category',
        name: 'category',
        component: '@/pages/content/category/archive',
      },
      {
        hideInMenu: true,
        path: '/archive/category/detail',
        component: '@/pages/content/category/archive/detail',
      },
      {
        path: '/archive/tag',
        name: 'tag',
        component: '@/pages/content/tag/index',
      },
      {
        hideInMenu: true,
        path: '/archive/tag/detail',
        component: '@/pages/content/tag/detail',
      },
      {
        path: '/archive/page',
        name: 'page',
        component: '@/pages/content/category/page',
      },
      {
        hideInMenu: true,
        path: '/archive/page/detail',
        component: '@/pages/content/category/page/detail',
      },
      {
        path: '/archive/module',
        name: 'module',
        component: '@/pages/content/module/index',
      },
      {
        path: '/archive/attachment',
        name: 'attachment',
        component: '@/pages/content/attachment/index',
      },
      {
        path: '/archive/place',
        name: 'place',
        component: '@/pages/content/place/index',
      },
      {
        hideInMenu: true,
        path: '/archive/place/detail',
        component: '@/pages/content/place/detail',
      },
    ],
  },
  {
    path: '/plugin',
    name: 'plugin',
    icon: 'appstore',
    access: 'canAdmin',
    routes: [
      {
        path: '/plugin/index',
        name: 'index',
        hideInMenu: true,
        component: '@/pages/plugin/index',
      },
      {
        path: '/plugin/seo',
        name: 'seo',
        type: 'seo',
        icon: 'user',
        routes: [
          {
            path: '/plugin/seo/rewrite',
            name: 'rewrite',
            type: 'system',
            component: '@/pages/plugin/rewrite/index',
            icon: 'rewrite',
          },
          {
            path: '/plugin/seo/push',
            name: 'push',
            type: 'normal',
            component: '@/pages/plugin/push/index',
            icon: 'push',
          },
          {
            path: '/plugin/seo/sitemap',
            name: 'sitemap',
            type: 'normal',
            component: '@/pages/plugin/sitemap/index',
            icon: 'sitemap',
          },
          {
            path: '/plugin/seo/robots',
            name: 'robots',
            type: 'normal',
            component: '@/pages/plugin/robots/index',
            icon: 'robots',
          },
          {
            path: '/plugin/seo/llms',
            name: 'llms',
            type: 'normal',
            component: '@/pages/plugin/llms/index',
            icon: 'llms',
          },
          {
            path: '/plugin/seo/jsonld',
            name: 'jsonld',
            type: 'system',
            component: '@/pages/plugin/jsonld/index',
            icon: 'jsonld',
          },
        ],
      },
      {
        path: '/plugin/interaction',
        name: 'interaction',
        type: 'interaction',
        icon: 'user',
        routes: [
          {
            path: '/plugin/interaction/comment',
            name: 'comment',
            type: 'normal',
            component: '@/pages/plugin/comment/index',
            icon: 'comment',
          },
          {
            path: '/plugin/interaction/guestbook',
            name: 'guestbook',
            type: 'normal',
            component: '@/pages/plugin/guestbook/index',
            icon: 'guestbook',
          },
        ],
      },
      {
        path: '/plugin/contentops',
        name: 'contentops',
        type: 'contentops',
        icon: 'user',
        routes: [
          {
            path: '/plugin/contentops/anchor',
            name: 'anchor',
            type: 'normal',
            component: '@/pages/plugin/anchor/index',
            icon: 'anchor',
          },
          {
            path: '/plugin/contentops/keyword',
            name: 'keyword',
            type: 'content',
            component: '@/pages/plugin/keyword/index',
            icon: 'keyword',
          },
          {
            path: '/plugin/contentops/material',
            name: 'material',
            type: 'content',
            component: '@/pages/plugin/material/index',
            icon: 'material',
          },
          {
            path: '/plugin/contentops/collector',
            name: 'collector',
            type: 'content',
            component: '@/pages/plugin/collector/index',
            icon: 'collector',
          },
          {
            path: '/plugin/contentops/importapi',
            name: 'importapi',
            type: 'content',
            component: '@/pages/plugin/importapi/index',
            icon: 'importapi',
          },
          {
            path: '/plugin/contentops/transfer',
            name: 'transfer',
            type: 'content',
            component: '@/pages/plugin/transfer/index',
            icon: 'transfer',
          },
          {
            path: '/plugin/contentops/titleimage',
            name: 'titleimage',
            type: 'content',
            component: '@/pages/plugin/titleimage/index',
            icon: 'titleimage',
          },
          {
            path: '/plugin/contentops/aigenerate',
            name: 'aigenerate',
            type: 'content',
            component: '@/pages/plugin/aigenerate/index',
            icon: 'ai',
          },
          {
            path: '/plugin/contentops/timefactor',
            name: 'timefactor',
            type: 'content',
            component: '@/pages/plugin/timefactor/index',
            icon: 'timefactor',
          },
        ],
      },
      {
        path: '/plugin/commerce',
        name: 'commerce',
        type: 'commerce',
        icon: 'order',
        routes: [
          {
            path: '/plugin/commerce/user',
            name: 'user',
            type: 'shop',
            component: '@/pages/plugin/user/index',
            icon: 'user',
          },
          {
            path: '/plugin/commerce/group',
            name: 'group',
            type: 'shop',
            component: '@/pages/plugin/group/index',
            icon: 'group',
          },
          {
            path: '/plugin/commerce/order',
            name: 'order',
            type: 'shop',
            component: '@/pages/plugin/order/index',
            icon: 'order',
          },
          {
            path: '/plugin/commerce/pay',
            name: 'pay',
            type: 'shop',
            component: '@/pages/plugin/pay/index',
            icon: 'pay',
          },
          {
            path: '/plugin/commerce/finance',
            name: 'finance',
            type: 'shop',
            component: '@/pages/plugin/finance/index',
            icon: 'finance',
          },
          {
            path: '/plugin/commerce/retailer',
            name: 'retailer',
            type: 'shop',
            component: '@/pages/plugin/retailer/index',
            icon: 'retailer',
          },
        ],
      },
      {
        path: '/plugin/channel',
        name: 'channel',
        type: 'channel',
        icon: 'user',
        routes: [
          {
            path: '/plugin/channel/sendmail',
            name: 'sendmail',
            type: 'system',
            component: '@/pages/plugin/sendmail/index',
            icon: 'sendmail',
          },
          {
            path: '/plugin/channel/subscription',
            name: 'subscription',
            type: 'system',
            component: '@/pages/plugin/subscription/index',
            icon: 'sendmail',
          },
          {
            path: '/plugin/channel/wechat',
            name: 'wechat',
            type: 'shop',
            component: '@/pages/plugin/wechat/index',
            icon: 'wechat',
          },
          {
            path: '/plugin/channel/weapp',
            name: 'weapp',
            type: 'shop',
            component: '@/pages/plugin/weapp/index',
            icon: 'weapp',
          },
          {
            path: '/plugin/channel/akismet',
            name: 'akismet',
            type: 'system',
            component: '@/pages/plugin/akismet/index',
            icon: 'akismet',
          },
          {
            path: '/plugin/channel/google',
            name: 'google',
            type: 'system',
            component: '@/pages/plugin/google/index',
            icon: 'google',
          },
        ],
      },
      {
        path: '/plugin/system',
        name: 'system',
        type: 'system',
        icon: 'system',
        routes: [
          {
            path: '/plugin/system/friendlink',
            name: 'friendlink',
            type: 'normal',
            component: '@/pages/plugin/link/index',
            icon: 'friendlink',
          },
          {
            path: '/plugin/system/fileupload',
            name: 'fileupload',
            type: 'normal',
            component: '@/pages/plugin/fileupload/index',
            icon: 'fileupload',
          },
          {
            path: '/plugin/system/redirect',
            name: 'redirect',
            type: 'content',
            component: '@/pages/plugin/redirect/index',
            icon: 'redirect',
          },
          {
            path: '/plugin/system/storage',
            name: 'storage',
            type: 'system',
            component: '@/pages/plugin/storage/index',
            icon: 'storage',
          },
          {
            path: '/plugin/system/fulltext',
            name: 'fulltext',
            type: 'content',
            component: '@/pages/plugin/fulltext/index',
            icon: 'fulltext',
          },
          {
            path: '/plugin/system/backup',
            name: 'backup',
            type: 'system',
            component: '@/pages/plugin/backup/index',
            icon: 'backup',
          },
          {
            path: '/plugin/system/replace',
            name: 'replace',
            type: 'system',
            component: '@/pages/plugin/replace/index',
            icon: 'replace',
          },
          {
            path: '/plugin/system/htmlcache',
            name: 'htmlcache',
            type: 'system',
            component: '@/pages/plugin/htmlcache/index',
            icon: 'htmlcache',
          },
          {
            path: '/plugin/system/limiter',
            name: 'limiter',
            type: 'system',
            component: '@/pages/plugin/limiter/index',
            icon: 'limiter',
          },
          {
            path: '/plugin/system/interference',
            name: 'interference',
            type: 'system',
            component: '@/pages/plugin/interference/index',
            icon: 'interference',
          },
          {
            path: '/plugin/system/watermark',
            name: 'watermark',
            type: 'content',
            component: '@/pages/plugin/watermark/index',
            icon: 'watermark',
          },
          {
            path: '/plugin/system/translate',
            name: 'translate',
            type: 'content',
            component: '@/pages/plugin/translate/index',
            icon: 'translate',
          },
        ],
      },
      {
        path: '/plugin/google',
        name: 'google',
        type: 'system',
        component: '@/pages/plugin/google/index',
        icon: 'google',
      },
      {
        path: '/plugin/llms',
        name: 'llms',
        type: 'normal',
        component: '@/pages/plugin/llms/index',
        icon: 'llms',
      },
    ],
  },
  {
    path: '/design',
    name: 'design',
    icon: 'Layout',
    access: 'canAdmin',
    routes: [
      {
        path: '/design/index',
        name: 'index',
        component: '@/pages/design/index',
      },
      {
        hideInMenu: true,
        path: '/design/editor',
        name: 'editor',
        component: '@/pages/design/editor',
      },
      {
        hideInMenu: true,
        path: '/design/detail',
        name: 'detail',
        component: '@/pages/design/detail',
      },
      {
        path: '/design/preview',
        name: 'ai-design',
        redirect: '/ai-design',
      },
      {
        path: '/design/doc',
        name: 'doc',
        component: '@/pages/design/doc',
      },
      {
        path: '/design/market',
        name: 'market',
        component: '@/pages/design/market',
      },
      {
        path: '/design/diyfield',
        name: 'diy',
        component: '@/pages/design/diyfield',
      },
    ],
  },
  {
    path: '/statistic',
    name: 'statistic',
    icon: 'lineChart',
    access: 'canAdmin',
    routes: [
      {
        path: '/statistic/liveview',
        name: 'liveview',
        component: '@/pages/statistic/liveview',
      },
      {
        path: '/statistic/conversion',
        name: 'conversion',
        component: '@/pages/statistic/conversion',
      },
      {
        path: '/statistic/orders',
        name: 'orders',
        component: '@/pages/statistic/orders',
      },
      {
        path: '/statistic/products',
        name: 'products',
        component: '@/pages/statistic/products',
      },
      {
        path: '/statistic/customers',
        name: 'customers',
        component: '@/pages/statistic/customers',
      },
      {
        path: '/statistic/search',
        name: 'search',
        component: '@/pages/statistic/search',
      },
      {
        hideInMenu: true,
        path: '/statistic/search/detail',
        name: 'searchdetail',
        component: '@/pages/statistic/searchDetail',
      },
      {
        path: '/statistic/coupons',
        name: 'coupons',
        component: '@/pages/statistic/coupons',
      },
      {
        path: '/statistic/spider',
        name: 'spider',
        component: '@/pages/statistic/spider',
      },
      {
        path: '/statistic/traffic',
        name: 'traffic',
        component: '@/pages/statistic/traffic',
      },
      {
        hideInMenu: true,
        path: '/statistic/detail',
        name: 'detail',
        component: '@/pages/statistic/detail',
      },
      {
        path: '/statistic/includes',
        name: 'includes',
        component: '@/pages/statistic/include',
      },
      {
        hideInMenu: true,
        path: '/statistic/include/detail',
        name: 'includedetail',
        component: '@/pages/statistic/include/detail',
      },
    ],
  },
  {
    name: 'account',
    icon: 'user',
    path: '/account',
    access: 'canAdmin',
    hideInTop: true,
    routes: [
      {
        path: '/account/index',
        name: 'index',
        component: '@/pages/account/index',
      },
      {
        path: '/account/list',
        name: 'list',
        component: '@/pages/account/list',
      },
      {
        path: '/account/group/list',
        name: 'grouplist',
        component: '@/pages/account/group/index',
      },
      {
        path: '/account/logs/login',
        name: 'loginlogs',
        component: '@/pages/account/logs/login',
      },
      {
        path: '/account/logs/action',
        name: 'actionlogs',
        component: '@/pages/account/logs/action',
      },
    ],
  },
  {
    path: '/website',
    name: 'website',
    icon: 'Cluster',
    access: 'canSuper',
    hideInTop: true,
    routes: [
      {
        path: '/website/list',
        name: 'website',
        component: '@/pages/website',
        icon: 'Cluster',
      },
      {
        path: '/website/multilang',
        name: 'multilang',
        component: '@/pages/plugin/multilang/index',
        icon: 'multilang',
      },
    ],
  },
  {
    path: '/tool/upgrade',
    icon: 'CloudSync',
    name: 'upgrade',
    access: 'canAdmin',
    hideInTop: true,
    component: '@/pages/tool/upgrade',
  },
  {
    path: '/tool/cache',
    name: 'cache',
    hideInMenu: true,
    component: '@/pages/tool/cache',
  },
  {
    path: '/preview',
    link: 'preview',
    name: 'preview',
    hideInMenu: true,
  },
  {
    path: '/ai-design',
    layout: false,
    hideInMenu: true,
    hideInTop: true,
    component: '@/pages/design/preview',
  },
  {
    path: '/',
    redirect: '/dashboard',
  },
  {
    path: '*',
    component: './404',
  },
];
