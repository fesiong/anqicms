// https://umijs.org/config/
import { defineConfig } from '@umijs/max';
const MonacoWebpackPlugin = require('monaco-editor-webpack-plugin');

import defaultSettings from './defaultSettings';
import routes from './routes';

let publicPath = '/system/';

if (typeof window !== 'undefined' && window.APP_CONFIG) {
  publicPath = window.APP_CONFIG.apiBaseUrl;
}

export default defineConfig({
  base: publicPath,
  publicPath: publicPath,
  /**
   * @name 开启 hash 模式
   * @description 让 build 之后的产物包含 hash 后缀。通常用于增量发布和避免浏览器加载缓存。
   * @doc https://umijs.org/docs/api/config#hash
   */
  hash: true,
  antd: {
    import: false,
  },
  /**
   * @name layout 插件
   * @doc https://umijs.org/docs/max/layout-menu
   */
  layout: {
    // https://umijs.org/zh-CN/plugins/plugin-layout
    locale: true,
    siderWidth: 208,
    ...defaultSettings,
  },
  /**
   * @name moment2dayjs 插件
   * @description 将项目中的 moment 替换为 dayjs
   * @doc https://umijs.org/docs/max/moment2dayjs
   */
  moment2dayjs: {
    preset: 'antd',
    plugins: ['duration'],
  },
  /**
   * @name 国际化插件
   * @doc https://umijs.org/docs/max/i18n
   */
  locale: {
    // default zh-CN
    default: 'zh-CN',
    antd: true,
    title: true,
    // default true, when it is true, will use `navigator.language` overwrite default
    baseNavigator: true,
    useLocalStorage: true,
  },
  /**
   * @name 权限插件
   * @description 基于 initialState 的权限插件，必须先打开 initialState
   * @doc https://umijs.org/docs/max/access
   */
  access: {},
  /**
   * @name <head> 中额外的 script
   * @description 配置 <head> 中额外的 script
   */
  // headScripts: [
  //   // 解决首次加载时白屏的问题
  //   { src: publicPath + 'scripts/loading.js', async: true },
  // ],
  /**
   * @name 兼容性设置
   * @description 设置 ie11 不一定完美兼容，需要检查自己使用的所有依赖
   * @doc https://umijs.org/docs/api/config#targets
   */
  // targets: {
  //   ie: 11,
  // },
  /**
   * @name 路由的配置，不在路由中引入的文件不会编译
   * @description 只支持 path，component，routes，redirect，wrappers，title 的配置
   * @doc https://umijs.org/docs/guides/routes
   */
  // umi routes: https://umijs.org/docs/routing
  routes,
  /**
   * @name 主题的配置
   * @description 虽然叫主题，但是其实只是 less 的变量设置
   * @doc antd的主题设置 https://ant.design/docs/react/customize-theme-cn
   * @doc umi 的theme 配置 https://umijs.org/docs/api/config#theme
   */
  theme: {
    // 如果不想要 configProvide 动态设置主题需要把这个设置为 default
    // 只有设置为 variable， 才能使用 configProvide 动态设置主色调
    'root-entry-name': 'variable',
  },
  title: 'AnQiCMS',
  /**
   * @name moment 的国际化配置
   * @description 如果对国际化没有要求，打开之后能减少js的包大小
   * @doc https://umijs.org/docs/api/config#ignoremomentlocale
   */
  ignoreMomentLocale: false,
  manifest: {
    basePath: publicPath,
  },
  /**
   * @name 快速热更新配置
   * @description 一个不错的热更新组件，更新时可以保留 state
   */
  fastRefresh: true,
  /**
   * @name 数据流插件
   * @@doc https://umijs.org/docs/max/data-flow
   */
  model: {},
  /**
   * 一个全局的初始数据流，可以用它在插件之间共享数据
   * @description 可以用来存放一些全局的数据，比如用户信息，或者一些全局的状态，全局初始状态在整个 Umi 项目的最开始创建。
   * @doc https://umijs.org/docs/max/data-flow#%E5%85%A8%E5%B1%80%E5%88%9D%E5%A7%8B%E7%8A%B6%E6%80%81
   */
  initialState: {},
  mfsu: {
    strategy: 'normal',
  },
  esbuildMinifyIIFE: true,
  /**
   * @name request 网络请求配置
   * @description 它基于 axios 和 ahooks 的 useRequest 提供了一套统一的网络请求和错误处理方案。
   * @doc https://umijs.org/docs/max/request
   */
  request: {},
  exportStatic: {
    ignorePreRenderError: true,
  },
  chainWebpack(config: any) {
    config.plugin('monaco-editor').use(MonacoWebpackPlugin, [
      {
        languages: ['javascript', 'css', 'html', 'markdown', 'yaml'],
      },
    ]);
    // webpack 5.88.2 InnerGraphPlugin bug (webpack#17711):
    // mis-optimizes `class extends Super` into `extends (null && Super)` in production builds,
    // causing "Super constructor null ... is not a constructor" at runtime.
    // Fixed in webpack 5.90; disable until umi bundles a newer webpack.
    config.optimization.set('innerGraph', false);
  },
});
