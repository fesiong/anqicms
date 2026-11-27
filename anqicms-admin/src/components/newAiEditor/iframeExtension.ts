import { Node, mergeAttributes } from '@tiptap/core';

/**
 * 通用 iframe 嵌入节点。
 *
 * 背景：aieditor 2.0 只内置了 youtube / twitch 两种嵌入节点，schema 里没有通用 iframe。
 * 于是「嵌入视频」弹窗里粘 B 站/优酷的 <iframe> 代码时，解析不出节点，
 * 整段 HTML 会被 tiptap 当纯文本塞进正文 —— 和 video 那个坑是同一类。
 *
 * 实现要点：
 * - `atom: true`：整体不可编辑，避免光标钻进 iframe 内部；
 * - 自建 NodeView：真实 <iframe> 会吞掉指针事件，导致节点选不中、删不掉，
 *   所以盖一层透明遮罩，点遮罩 = 选中节点；节点被选中时（ProseMirror 会加
 *   `ProseMirror-selectednode`）由 CSS 隐藏遮罩，iframe 才可交互（见 index.less）。
 */

const DEFAULT_WIDTH = '100%';
const DEFAULT_HEIGHT = '480px';

/** iframe 的尺寸属性既可能是 480 / 100% 也可能带单位，统一成 CSS 值 */
export function normalizeSize(value: unknown, fallback: string): string {
  if (typeof value === 'number' && Number.isFinite(value)) return `${value}px`;
  if (typeof value !== 'string') return fallback;
  const trimmed = value.trim();
  if (!trimmed) return fallback;
  if (/^\d+(\.\d+)?$/.test(trimmed)) return `${trimmed}px`;
  return trimmed;
}

/**
 * iframe 只接受绝对地址或站内相对路径。
 * 后台虽然是站长自己用，但 javascript: / data: 这类协议没有任何嵌入理由，
 * 一旦进了正文就是存储型 XSS 的入口，插之前就拦掉。
 */
export function isSafeEmbedSrc(src: string): boolean {
  const value = src.trim();
  if (!value) return false;
  return /^(https?:\/\/|\/\/|\/)/i.test(value);
}

/** 从一段嵌入代码里取出 iframe（取第一个） */
export function parseIframeHtml(
  html: string,
): { src: string; width?: string; height?: string; title?: string } | null {
  const match = html.match(/<iframe\b[^>]*>/i);
  if (!match) return null;
  const tag = match[0];
  const attr = (name: string): string | undefined => {
    const found = tag.match(
      new RegExp(`\\b${name}\\s*=\\s*("([^"]*)"|'([^']*)'|([^\\s>]+))`, 'i'),
    );
    if (!found) return undefined;
    return found[2] ?? found[3] ?? found[4];
  };
  const src = attr('src');
  if (!src) return null;
  return {
    src,
    width: attr('width'),
    height: attr('height'),
    title: attr('title'),
  };
}

export const Iframe = Node.create({
  name: 'iframe',
  group: 'block',
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      src: { default: null },
      title: { default: null },
      width: { default: DEFAULT_WIDTH },
      height: { default: DEFAULT_HEIGHT },
      allow: { default: null },
      allowfullscreen: { default: 'true' },
      frameborder: { default: '0' },
      scrolling: { default: null },
    };
  },

  parseHTML() {
    // 只认 src；youtube / twitch 的嵌入节点是独立节点名，不会走到这里
    return [{ tag: 'iframe[src]' }];
  },

  renderHTML({ HTMLAttributes }) {
    // width/height 属性在 iframe 上不认 px/百分比混写，统一转成 style，
    // 前台模板直接渲染也不会塌成 0 高度
    const { width, height, ...rest } = HTMLAttributes as Record<string, any>;
    const box = `width:${normalizeSize(
      width,
      DEFAULT_WIDTH,
    )};height:${normalizeSize(height, DEFAULT_HEIGHT)};border:0;`;
    return [
      'iframe',
      mergeAttributes(rest, {
        frameborder: rest.frameborder ?? '0',
        allowfullscreen: rest.allowfullscreen ?? 'true',
        style: box,
      }),
    ];
  },

  addNodeView() {
    return ({ node }) => {
      const wrapper = document.createElement('div');
      wrapper.className = 'anqi-iframe-wrapper';
      wrapper.style.height = normalizeSize(node.attrs.height, DEFAULT_HEIGHT);

      const frame = document.createElement('iframe');
      frame.className = 'anqi-iframe-frame';
      frame.setAttribute('frameborder', '0');
      frame.setAttribute('allowfullscreen', 'true');
      frame.setAttribute('loading', 'lazy');
      const syncFrame = (attrs: Record<string, any>) => {
        frame.src = attrs.src ?? '';
        if (attrs.title) frame.title = attrs.title;
        else frame.removeAttribute('title');
        if (attrs.allow) frame.setAttribute('allow', attrs.allow);
        wrapper.style.height = normalizeSize(attrs.height, DEFAULT_HEIGHT);
      };
      syncFrame(node.attrs);

      // 遮罩：未选中时挡住 iframe，点击等于选中节点（否则点不到、删不掉）
      const mask = document.createElement('div');
      mask.className = 'anqi-iframe-mask';

      wrapper.append(frame, mask);

      return {
        dom: wrapper,
        update: (updated: any) => {
          if (updated.type.name !== this.name) return false;
          syncFrame(updated.attrs);
          return true;
        },
      };
    };
  },
});
