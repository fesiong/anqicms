/**
 * 顶部搜索的菜单索引与匹配逻辑（纯函数，不依赖 React，方便单测）。
 *
 * 背景：菜单从两层变成三层后（`/plugin` → `/plugin/seo` → `/plugin/seo/rewrite`），
 * 原来「先看有没有 routes、有就只取第二层」的写法会把分组节点（不可跳转）当成结果，
 * 而真正的三级功能页永远进不了候选。这里改成不限深度的递归收集。
 */

export type MenuRoute = {
  path?: string;
  name?: string;
  routes?: MenuRoute[];
  /** 不在菜单里展示（含其整棵子树） */
  hideInMenu?: boolean;
  /** 独立页面（登录页等），不属于后台功能 */
  layout?: boolean;
  component?: unknown;
  redirect?: string;
};

export type SearchItem = {
  /** 跳转地址，同时作为候选项的唯一 value */
  path: string;
  /** 叶子节点自身的标题 */
  title: string;
  /** 从一级菜单到直接父级的标题链（不含自身） */
  parents: string[];
};

/**
 * 收集所有可搜索的功能页。
 *
 * 只收「真正的叶子页面」：有 component、无 redirect、有 path。
 * 分组节点（只有 routes 没有 component）和重定向节点会被跳过，
 * 因为它们点进去是空页或直接跳走，放进搜索结果只会让人踩空。
 *
 * @param routes 路由配置树
 * @param resolveTitle 传入一段时间链（如 ['plugin','seo','rewrite']），返回本地化标题
 */
export const collectSearchItems = (
  routes: MenuRoute[],
  resolveTitle: (names: string[]) => string,
): SearchItem[] => {
  const items: SearchItem[] = [];

  const walk = (list: MenuRoute[], names: string[], titles: string[]) => {
    for (const item of list || []) {
      if (!item || item.hideInMenu || item.layout === false) continue;

      const nextNames = item.name ? [...names, item.name] : names;
      const nextTitles = item.name ? [...titles, resolveTitle(nextNames)] : titles;
      const children = Array.isArray(item.routes) ? item.routes : [];

      if (children.length) {
        walk(children, nextNames, nextTitles);
        continue;
      }

      if (!item.path || !item.component || item.redirect) continue;
      if (!nextTitles.length) continue;

      items.push({
        path: item.path,
        title: nextTitles[nextTitles.length - 1],
        parents: nextTitles.slice(0, -1),
      });
    }
  };

  walk(routes, [], []);
  return items;
};

const includesKeyword = (text: string, keyword: string) =>
  text.toLowerCase().indexOf(keyword) !== -1;

/**
 * 按关键词过滤并排序。
 *
 * 上级菜单标题也参与匹配：搜「SEO」能列出该分类下的全部功能——这正是菜单多出一级
 * 之后最需要的能力（用户常常记得大分类、记不住具体功能名）。
 *
 * 排序让「标题前缀命中」优先，避免搜「SEO」时标题里带 SEO 的页面被分类命中项挤下去。
 */
export const matchSearchItems = (
  items: SearchItem[],
  keyword?: string,
): SearchItem[] => {
  const kw = (keyword || '').trim().toLowerCase();
  if (!kw) return items;

  const hits: { item: SearchItem; rank: number }[] = [];
  for (const item of items) {
    const titleIndex = item.title.toLowerCase().indexOf(kw);
    const pathHit = includesKeyword(item.path, kw);
    const parentHit = item.parents.some((p) => includesKeyword(p, kw));

    if (titleIndex === -1 && !pathHit && !parentHit) continue;

    // 0 标题前缀 → 1 标题包含 → 2 路径命中 → 3 仅上级分类命中
    let rank = 3;
    if (titleIndex === 0) rank = 0;
    else if (titleIndex > 0) rank = 1;
    else if (pathHit) rank = 2;

    hits.push({ item, rank });
  }

  hits.sort((a, b) => a.rank - b.rank || a.item.title.length - b.item.title.length);
  return hits.map((h) => h.item);
};
