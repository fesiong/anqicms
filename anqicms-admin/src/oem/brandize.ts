import { brand } from './brand';

function hostOf(website: string): string {
  if (!website) {
    return '';
  }
  try {
    // 去掉 www.：locales 里既有 www.anqicms.com 也有 m./admin. 子域，
    // 整段换掉 www. 之后剩下的子域靠主机名替换，带 www. 会拼出
    // admin.www.acme.test 这种不存在的地址。
    return new URL(website).host.replace(/^www\./, '');
  } catch {
    return '';
  }
}

/**
 * 把文案里的官方品牌名换成当前品牌。
 *
 * 为什么在运行时替换而不是改 locale 源文件：品牌文案散落在 zh-CN / zh-TW / en-US
 * 三套语言、几十个 key 里（登录页、页脚、仪表盘引导、AI 提示…），逐个改成品牌值
 * 意味着每加一个 OEM 都要改三遍 74 行，且上游新增文案时一定会漏。
 * 在语言包出口统一替换，只需要一个函数，新文案自动继承。
 *
 * 官方版原样返回，逐字节不变，因此对主站零影响。
 * 后端 config.BrandizeWith 是同一套规则，两侧替换结果必须一致。
 */
export function brandize(text: string): string {
  if (!text || brand.official) {
    return text;
  }
  let s = text;
  // 先整段换地址，再换名字。反过来会把 "https://www.anqicms.com" 拆成
  // "https://www.acme"，变成打不开的地址。
  if (brand.authBase) {
    s = s
      .split('https://auth.anqicms.com/auth')
      .join(brand.authBase.replace(/\/$/, ''));
  }
  if (brand.website) {
    s = s
      .split('https://www.anqicms.com')
      .join(brand.website.replace(/\/$/, ''));
  }
  const host = hostOf(brand.website);
  if (host) {
    s = s.split('anqicms.com').join(host);
  }
  // 顺序要紧：「安企CMS」必须排在「安企」前面，
  // 否则先命中短词会剩下一截「CMS」粘在品牌名后面。
  return s
    .split('AnQiCMS')
    .join(brand.name)
    .split('AnqiCMS')
    .join(brand.name)
    .split('安企CMS')
    .join(brand.nameCn)
    .split('安企')
    .join(brand.nameCn)
    .split('anqicms')
    .join(brand.id);
}

/**
 * 对整份语言包做品牌替换。
 *
 * 只替换 value 不替换 key：key 是代码里的查表标识（例如 `design.share.nologin`），
 * 动 key 会让所有 `intl.formatMessage({id})` 找不到词条，直接显示成 key 本身。
 */
export function brandizeMessages<T extends Record<string, any>>(messages: T): T {
  if (brand.official) {
    return messages;
  }
  const out: Record<string, any> = {};
  Object.keys(messages).forEach((key) => {
    const value = messages[key];
    out[key] = typeof value === 'string' ? brandize(value) : value;
  });
  return out as T;
}

export default brandize;
