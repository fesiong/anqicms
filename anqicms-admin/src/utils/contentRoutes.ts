/**
 * 仪表盘等内容入口的路由分流。
 *
 * 背景：仪表盘的「文档量」「快捷操作」统计的是**全部模型**，但跳转目标一直写死
 * `/product/*`。普通内容模式下 `/product` 属于 SHOP_TOP_GROUPS，整组菜单被隐藏
 * （见 utils/shopMode + app.tsx 的 menuDataRender），点进去就是不可达页面；
 * 商城模式下产品又归商品管理页 `/product/index` 管。
 *
 * 模式判断复用 utils/shopMode 的运行期缓存，和菜单过滤读同一个信号，
 * 避免出现「菜单藏了、链接还往那儿跳」的不一致。
 */
import { getRuntimeShopMode } from './shopMode';

/** 后端 archive_counts 里的一项，kind 来自 modules.kind（article / product） */
export type ContentModuleItem = {
  id?: number;
  kind?: string;
};

/** 文档列表（全部模型），status 可选 ok / draft / plan */
const archiveList = (status?: string, moduleId?: number) => {
  const query: string[] = [];
  if (status) query.push('status=' + status);
  if (moduleId !== undefined) query.push('module_id=' + moduleId);
  return '/archive/list' + (query.length ? '?' + query.join('&') : '');
};

/**
 * 某个模型的列表页。
 * 商城模式 + 产品模型 → 商品管理页（该页只展示产品，不带 module_id）；
 * 其余一律文档列表并定位到该模型——普通模式下产品也在文档列表里。
 */
export const moduleListPath = (item?: ContentModuleItem): string => {
  if (getRuntimeShopMode() && item?.kind === 'product') {
    return '/product/index';
  }
  return archiveList(undefined, item?.id);
};

/**
 * 「待发布 / 草稿」这类跨模型汇总数字的入口。
 * 汇总口径含产品，而两种模式下都没有一个页面能同时列出全部模型的草稿，
 * 取两种模式都可达、且在普通模式下口径完全一致的文档列表。
 */
export const contentListPath = (status: 'draft' | 'plan'): string => {
  return archiveList(status);
};

/** 新建内容的编辑器入口 */
export const contentEditPath = (): string => {
  return getRuntimeShopMode() ? '/product/detail' : '/archive/detail';
};
