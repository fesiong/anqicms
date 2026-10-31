/**
 * 站点模式（shop_mode）相关的菜单过滤规则。
 *
 * 单一来源：`app.tsx` 的 menuDataRender（侧栏）与 `headerContent` 的顶部导航
 * 都必须走这里，否则会出现「侧栏隐藏了商城菜单、顶部导航还挂着」的不一致。
 *
 * 新增商城顶级分组 / 组内商城子页时，只改这两个清单。
 */

/** 商城专属的顶级导航分组：普通模式整组隐藏 */
export const SHOP_TOP_GROUPS = [
  '/order',
  '/product',
  '/customer',
  '/promotion',
  '/sale',
];

/** 挂在共享分组（如 /statistic、/setting）下的商城子页：普通模式按路径隐藏 */
export const SHOP_PATHS = [
  '/statistic/liveview',
  '/statistic/conversion',
  '/statistic/orders',
  '/statistic/products',
  '/statistic/customers',
  '/statistic/search',
  '/statistic/coupons',
  '/setting/currency',
  '/setting/pay',
];

/** 是否商城模式（只有显式 1 才算，其余一律按普通内容模式处理） */
export const isShopMode = (system?: { shop_mode?: number }): boolean =>
  system?.shop_mode === 1;

/**
 * 商城模式把「设置」「功能」收进了侧栏，顶栏不再显示；
 * 普通内容模式沿用主干，把两者放在顶栏。
 */
export const SHOP_MODE_HIDE_IN_TOP = ['/setting', '/plugin'];

/**
 * 运行期缓存：给无法使用 hooks 的类组件（archive/detail.tsx 等）读取。
 * 由 app.tsx 的 applyShopModeTheme 在 getInitialState 与 layout 运行期写入。
 */
let runtimeShopMode = false;

export const setRuntimeShopMode = (system?: {
  shop_mode?: number;
}): boolean => {
  runtimeShopMode = isShopMode(system);
  return runtimeShopMode;
};

export const getRuntimeShopMode = (): boolean => runtimeShopMode;

/**
 * 普通模式下某个路由是否属于商城菜单（顶级分组或商城子页）。
 * @param path 路由 path
 * @param isTopLevel 是否顶级分组（顶级只看 SHOP_TOP_GROUPS）
 */
export const isShopMenuPath = (path?: string, isTopLevel = false): boolean => {
  if (!path) return false;
  if (SHOP_TOP_GROUPS.indexOf(path) !== -1) return true;
  if (isTopLevel) return false;
  return SHOP_PATHS.some((p) => path === p || path.startsWith(p + '/'));
};
