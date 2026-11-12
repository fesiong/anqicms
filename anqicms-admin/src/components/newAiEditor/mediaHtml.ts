/**
 * 富文本里 video 的 HTML 生成与归一化。
 *
 * ⚠️ aieditor 的 video 节点 parseHTML 只有一条规则：`video[src]`
 *    （audio 同款：`audio[src]`）。也就是 src 必须**直接写在标签上**。
 *    旧代码生成的是 `<video poster="..."><source src="..."></video>` —— video 标签上没有 src，
 *    解析不中任何节点，tiptap 就把这段 HTML 当纯文本塞进正文，
 *    界面上表现为「插入视频后正文里直接显示源码」。
 *
 * 因此：新内容统一从这里生成（src 在 video 上）；老内容在载入时用
 * normalizeVideoHtml 归一化，否则打开即变源码、再保存就彻底丢视频节点。
 */

const IMAGE_EXT_RE = /\.(?:jpe?g|png|gif|webp|bmp|avif|svg)(?:[?#].*)?$/i;
const VIDEO_EXT_RE = /\.(?:mp4|webm|ogv|ogg|mov|m4v)(?:[?#].*)?$/i;

export function isImageUrl(url?: string | null): boolean {
  if (!url) return false;
  return IMAGE_EXT_RE.test(url.trim());
}

export function isVideoUrl(url?: string | null): boolean {
  if (!url) return false;
  return VIDEO_EXT_RE.test(url.trim());
}

function escapeAttr(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

/**
 * 生成能被 editor 解析成 video 节点的 HTML。
 * poster 只有在确实是图片地址时才输出 —— 附件的 logo 字段对非图片文件
 * 会被后端回填成文件自身地址（视频的 logo 就是 mp4），把它当封面会让浏览器
 * 拿 mp4 去解码海报。
 */
export function buildVideoHtml(src: string, poster?: string | null): string {
  const attrs = [
    `src="${escapeAttr(src)}"`,
    'controls="controls"',
    'controlslist="nodownload"',
  ];
  if (isImageUrl(poster)) {
    attrs.push(`poster="${escapeAttr((poster as string).trim())}"`);
  }
  return `<video ${attrs.join(' ')}></video>`;
}

const VIDEO_TAG_RE = /<video\b([^>]*)>([\s\S]*?)<\/video\s*>/gi;
const SOURCE_SRC_RE =
  /<source\b[^>]*\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/i;
// 已经被文本化、存进库的 video：形如 &lt;video ...&gt;...&lt;/video&gt;
// 里面的 <source> 同样是转义的（&lt;source ...），所以 body 允许出现 &lt;，
// 但一旦撞上段落/块级闭合标签就说明跨段了，不能吃进来
// （video 与 iframe 是同一类 block 节点，都会被 tiptap 文本化，一起反解）
const ESCAPED_MEDIA_RE =
  /&lt;(video|iframe)\b((?:(?!&lt;\/(?:p|div|li|ul|ol|h[1-6]|td|th|tr|blockquote|pre)\b)[\s\S])*?)&lt;\/\1&gt;/gi;
// <pre>/<code> 里是有意展示的示例代码，不能反解
const PROTECTED_BLOCK_RE = /<(pre|code)\b[^>]*>[\s\S]*?<\/\1>/gi;

function decodeEscapedText(text: string): string {
  return text
    .replace(/&quot;/g, '"')
    .replace(/&#0*39;|&apos;/g, "'")
    .replace(/&gt;/g, '>')
    .replace(/&lt;/g, '<')
    .replace(/&amp;/g, '&');
}

/**
 * 把已经存成文本的 video / iframe（`&lt;video ...`）反解回标签。
 * 这些是修复前插入产生的坏数据：正文里直接躺着一整段被转义的源码，
 * 不反解的话用户打开文章看到的还是源码，等于没修。
 */
export function restoreEscapedMediaHtml(html: string): string {
  if (!html || !/&lt;(?:video|iframe)/i.test(html)) return html;
  const stash: string[] = [];
  const guarded = html.replace(PROTECTED_BLOCK_RE, (block) => {
    stash.push(block);
    return `\u0000${stash.length - 1}\u0000`;
  });
  const restored = guarded.replace(
    ESCAPED_MEDIA_RE,
    (_match: string, tag: string, body: string) =>
      // video / iframe 在 schema 里都是 block 节点，直接塞在 <p> 里会被整段文本化，
      // 所以反解时先闭合当前段落、余下文本另起一段（HTML 解析器会自愈嵌套）
      `</p><${tag}${decodeEscapedText(body)}</${tag}><p>`,
  );
  if (!stash.length) return restored;
  return restored.replace(
    /\u0000(\d+)\u0000/g,
    (_match: string, index: string) => stash[Number(index)] ?? '',
  );
}

/**
 * 把 `<video ...><source src="x"></video>` 归一化成 `<video ... src="x"></video>`。
 * 顺手反解已被文本化的历史内容。已经带 src 的、以及完全取不到地址的一律原样返回。
 */
export function normalizeVideoHtml(html: string): string {
  if (!html) return html;
  const restored = restoreEscapedMediaHtml(html);
  if (!/<video/i.test(restored)) return restored;
  return restored.replace(
    VIDEO_TAG_RE,
    (match: string, attrs: string, inner: string) => {
      if (/\bsrc\s*=/i.test(attrs)) return match;
      const source = inner.match(SOURCE_SRC_RE);
      const src = source ? source[1] ?? source[2] ?? source[3] ?? '' : '';
      if (!src) return match;
      return `<video${attrs} src="${escapeAttr(src)}"></video>`;
    },
  );
}

/**
 * 写回正文前的统一清理：
 * 1. 去掉 a 标签的 rel（编辑器会自动补 rel="noopener..."，存库无意义）；
 * 2. 给 video 补 controlslist="nodownload"（video 扩展的 renderHTML 只输出
 *    controls/playsinline/preload，schema 里没有 controlslist 属性，
 *    不补的话防下载提示会在每次保存后丢失）。
 */
export function sanitizeEditorHtml(html: string): string {
  let out = html.replace(/<a(\s[^>]*)?>/gi, (match) =>
    match.replace(/\srel\s*=\s*["'][^"']*["']/gi, ''),
  );
  out = out.replace(/<video\b([^>]*)>/gi, (match, attrs: string) =>
    /\bcontrolslist\s*=/i.test(attrs)
      ? match
      : `<video${attrs} controlslist="nodownload">`,
  );
  return out;
}
