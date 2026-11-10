import { uploadAttachment } from '@/services';
import { calculateFileMd5 } from '@/utils';
import { message } from 'antd';
import Editor, {
  BtnBold,
  BtnBulletList,
  BtnClearFormatting,
  BtnItalic,
  BtnLink,
  BtnNumberedList,
  BtnRedo,
  BtnStrikeThrough,
  BtnStyles,
  BtnUnderline,
  BtnUndo,
  createButton,
  HtmlButton,
  Separator,
  Toolbar,
} from 'react-simple-wysiwyg';
import TurndownService from 'turndown';

import { useIntl } from '@umijs/max';
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from 'react';
import AiEditorOverlay from '../aiEditorOverlay';
import Attachment from '../attachment/dialog';
import './index.less';

export type SimpleEditorProps = {
  className: string;
  content: string;
  setContent: (html: any) => Promise<void>;
  ref: any;
};

const IMAGE_EXT = ['webp', 'bmp', 'png', 'gif', 'jpg', 'jpeg', 'svg'];
const VIDEO_EXT = ['mp4', 'ogg', 'webm'];
const AUDIO_EXT = ['mp3', 'wav'];

const getAttachmentExt = (item: any) => {
  const name = String(item.file_location || item.file_name || '');
  const dot = name.lastIndexOf('.');
  return dot < 0 ? '' : name.slice(dot + 1).toLowerCase();
};

const escapeAttr = (value: any) =>
  String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

const buildEmbedHtml = (item: any) => {
  const ext = getAttachmentExt(item);
  const name = escapeAttr(item.file_name);
  const path = escapeAttr(item.file_path);
  if (item.is_image === 1 || IMAGE_EXT.includes(ext)) {
    // img
    return `<div class="content-image"><img src="${path}" alt="${name}" /></div>`;
  } else if (item.is_image === 2 || VIDEO_EXT.includes(ext)) {
    return `<video controls="controls" controlslist="nodownload" poster="${escapeAttr(
      item.logo,
    )}"><source src="${path}" type="video/${ext}">您的浏览器不支持 video 标签。</video>`;
  } else if (AUDIO_EXT.includes(ext)) {
    return `<audio src="${path}" controls="controls"><source src="${path}" type="audio/${ext}">你的编辑器不支持 audio 标签</audio>`;
  } else {
    return `<a href="${path}">${name}</a>`;
  }
};

// The AI panel speaks markdown: it receives markdown and answers with markdown,
// which aiEditorOverlay converts back to HTML. The editor holds HTML, so both the
// selection and the whole document go through turndown to keep headings, lists,
// links and emphasis instead of arriving as one flattened paragraph.
const turndownService = new TurndownService({
  headingStyle: 'atx',
  codeBlockStyle: 'fenced',
  bulletListMarker: '-',
  hr: '---',
});

turndownService.addRule('strikethrough', {
  // STRIKE is an obsolete tag, so the editor's strike-through button may emit any
  // of these three spellings.
  filter: (node) => ['DEL', 'S', 'STRIKE'].includes(node.nodeName),
  replacement: (content) => `~~${content}~~`,
});

// Neither media embeds nor underline have markdown syntax, and the attachment
// dialog writes them into the markdown editor as raw tags too, so keep the tags.
turndownService.keep(['u', 'ins', 'video', 'audio']);

const htmlToMarkdown = (html: string) => turndownService.turndown(html);

const rangeToMarkdown = (range: Range) => {
  const holder = document.createElement('div');
  holder.appendChild(range.cloneContents());
  return htmlToMarkdown(holder.innerHTML);
};

// 清理 HTML：正文里常见的脏来源是 Word/网页粘贴和旧编辑器留下的内联样式。
const DROP_TAGS = [
  'SCRIPT',
  'STYLE',
  'LINK',
  'META',
  'BASE',
  'NOSCRIPT',
  'IFRAME',
  'OBJECT',
  'EMBED',
  'FORM',
  'SVG',
  'MATH',
  'TITLE',
  // Word 导出的 <xml> 里是 VML 命名空间声明，整块丢弃。
  'XML',
];
// 只承载样式的容器，去掉标签本身保留内容。删除线/上标等是内容语义，不动。
const UNWRAP_TAGS = ['FONT', 'SPAN', 'BIG', 'CENTER', 'MARQUEE', 'LABEL'];
// 结构类属性一律丢弃，只留排版和内容真正需要的。
const KEEP_ATTRS = [
  'alt',
  'title',
  'href',
  'src',
  'target',
  'rel',
  'colspan',
  'rowspan',
  'start',
  'reversed',
  'type',
  'controls',
  'controlslist',
  'poster',
  'width',
  'height',
  'srcset',
];
// 空标签通常是被掏空的容器，但这些本来就不装文字。
const KEEP_WHEN_EMPTY = ['BR', 'HR', 'IMG', 'VIDEO', 'AUDIO', 'SOURCE'];

const isSafeUrl = (value: string) => {
  const url = value.replace(/[^\x21-\x7e]/g, '').toLowerCase();
  return (
    !/^(javascript|vbscript|file|blob):/.test(url) &&
    (!url.startsWith('data:') || /^data:image\/(png|jpe?g|gif|webp);/.test(url))
  );
};

const cleanElement = (node: Element) => {
  const tag = node.tagName.toUpperCase();
  if (DROP_TAGS.includes(tag)) {
    node.remove();
    return;
  }
  Array.from(node.childNodes).forEach(cleanChild);
  Array.from(node.attributes).forEach((attr) => {
    const name = attr.name.toLowerCase();
    // content-image 是模板的取图钩子，code 上的 class 是代码块语言标记，都得留下。
    const keepClass =
      name === 'class' &&
      (node.classList.contains('content-image') || tag === 'CODE');
    if (!keepClass && !KEEP_ATTRS.includes(name)) {
      node.removeAttribute(attr.name);
    } else if (
      ['href', 'src', 'poster', 'srcset'].includes(name) &&
      !isSafeUrl(attr.value)
    ) {
      node.removeAttribute(attr.name);
    }
  });
  // Word 粘贴的命名空间标签（o:p、v:shape 之类）只是样式壳子，剥壳留文字；
  // 直接删掉会把 <o:p> 里的正文一起丢掉。
  if (UNWRAP_TAGS.includes(tag) || tag.includes(':')) {
    const parent = node.parentNode;
    while (node.firstChild) {
      parent?.insertBefore(node.firstChild, node);
    }
    node.remove();
    return;
  }
  // Word 爱用 <p>&nbsp;</p> 占位，清完属性/子节点后这类空壳一并去掉；
  // 但 <p><br></p> 这种有元素子节点的换行段是有效排版，保留。
  const hasContent =
    node.children.length > 0 ||
    (node.textContent || '').replace(/[\u00a0\u200b]/g, ' ').trim() !== '';
  if (!KEEP_WHEN_EMPTY.includes(tag) && !hasContent) {
    node.remove();
  }
};

const cleanChild = (child: Node) => {
  if (child.nodeType === Node.COMMENT_NODE) {
    child.parentNode?.removeChild(child);
  } else if (child.nodeType === Node.ELEMENT_NODE) {
    cleanElement(child as Element);
  }
};

// DOMParser keeps the fragment inert: no resource loads and no handler firing.
const cleanHtml = (html: string) => {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  Array.from(doc.body.childNodes).forEach(cleanChild);
  return doc.body.innerHTML;
};

const MEDIA_ICON = (
  <svg
    className="icon"
    viewBox="0 0 1024 1024"
    version="1.1"
    xmlns="http://www.w3.org/2000/svg"
    width="16"
    height="16"
  >
    <path
      d="M416 266.538667m-96 0a96 96 0 1 0 192 0 96 96 0 1 0-192 0Z"
      fill="#010000"
    ></path>
    <path
      d="M721.749333 371.626667A43.818667 43.818667 0 0 0 682.666667 348.074667a42.965333 42.965333 0 0 0-38.058667 25.002666l-66.474667 146.517334a10.624 10.624 0 0 1-18.005333 2.261333l-34.986667-43.690667a42.666667 42.666667 0 0 0-34.688-16.042666 42.965333 42.965333 0 0 0-33.578666 18.176L323.84 670.293333a21.333333 21.333333 0 0 0 17.493333 33.706667h512a21.333333 21.333333 0 0 0 18.133334-10.112 21.333333 21.333333 0 0 0 0.938666-20.736z"
      fill="#010000"
    ></path>
    <path
      d="M938.666667 0H234.666667a85.333333 85.333333 0 0 0-85.333334 85.333333v704a85.333333 85.333333 0 0 0 85.333334 85.333334H938.666667a85.333333 85.333333 0 0 0 85.333333-85.333334V85.333333a85.333333 85.333333 0 0 0-85.333333-85.333333z m-6.186667 783.104a21.333333 21.333333 0 0 1-15.104 6.229333H256a21.333333 21.333333 0 0 1-21.333333-21.333333V106.666667A21.333333 21.333333 0 0 1 256 85.333333h661.333333a21.333333 21.333333 0 0 1 21.333334 21.333334V768a21.333333 21.333333 0 0 1-6.186667 14.976z"
      fill="#010000"
    ></path>
    <path
      d="M832 938.666667h-725.333333a21.333333 21.333333 0 0 1-21.333334-21.333334v-725.333333a42.666667 42.666667 0 0 0-85.333333 0V938.666667a85.333333 85.333333 0 0 0 85.333333 85.333333h746.666667a42.666667 42.666667 0 0 0 0-85.333333z"
      fill="#010000"
    ></path>
  </svg>
);

const SimpleEditor: React.FC<SimpleEditorProps> = forwardRef((props, ref) => {
  const intl = useIntl();
  // The memoized buttons below outlive individual renders, so they read intl
  // through a ref instead of closing over one render's object.
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [aiVisible, setAiVisible] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const editorRef = useRef<HTMLDivElement>(null);
  const selectionRef = useRef<Range | null>(null);

  // Remember the last caret/selection inside the editor: opening the attachment
  // dialog or uploading a dropped file moves focus away and clears it.
  useEffect(() => {
    const onSelectionChange = () => {
      const sel = window.getSelection();
      const range = sel && sel.rangeCount > 0 ? sel.getRangeAt(0) : null;
      if (range && editorRef.current?.contains(range.commonAncestorContainer)) {
        selectionRef.current = range.cloneRange();
      }
    };
    document.addEventListener('selectionchange', onSelectionChange);
    return () =>
      document.removeEventListener('selectionchange', onSelectionChange);
  }, []);

  // Media is inserted as sibling blocks of the block holding the caret.
  // document.execCommand('insertHTML') is unusable here: it unwraps the
  // content-image div and copies computed styles onto the img.
  const insertBlockHtml = useCallback((html: string) => {
    const el = editorRef.current;
    if (!html || !el || !el.isConnected) {
      return;
    }
    el.focus();

    let range = selectionRef.current;
    if (!range || !el.contains(range.commonAncestorContainer)) {
      range = null;
    }
    range?.deleteContents();

    let block: Node | null = null;
    if (range) {
      block = range.startContainer;
      while (block && block.parentNode !== el) {
        block = block.parentNode;
      }
    }

    const holder = document.createElement('div');
    holder.innerHTML = html;
    const nextSibling = block ? block.nextSibling : null;
    let last: Node | null = null;
    Array.from(holder.childNodes).forEach((node) => {
      last = el.insertBefore(node, nextSibling);
    });

    // execCommand fired 'input' for us; manual DOM writes do not.
    el.dispatchEvent(new InputEvent('input', { bubbles: true }));

    if (last) {
      const after = document.createRange();
      after.setStartAfter(last);
      after.collapse(true);
      selectionRef.current = after.cloneRange();
      const sel = window.getSelection();
      sel?.removeAllRanges();
      sel?.addRange(after);
    } else {
      selectionRef.current = null;
    }
  }, []);

  const openAttachment = useCallback(() => {
    Attachment.show(true, intlRef.current)
      .then((res: any) => {
        insertBlockHtml((res || []).map(buildEmbedHtml).join(''));
      })
      .catch(() => {});
  }, [insertBlockHtml]);

  const openAiPanel = useCallback(() => setAiVisible(true), []);

  const cleanEditorHtml = useCallback(() => {
    const el = editorRef.current;
    if (!el || !el.isConnected) {
      return;
    }
    const original = el.innerHTML;
    const cleaned = cleanHtml(original);
    if (!cleaned) {
      message.info(
        intlRef.current.formatMessage({
          id: 'component.simpleEditor.no-content-to-clean',
        }),
      );
      return;
    }
    if (cleaned === original) {
      message.info(
        intlRef.current.formatMessage({
          id: 'component.simpleEditor.already-clean',
        }),
      );
      return;
    }
    el.innerHTML = cleaned;
    // 受控 value 只在字符串不同时回写 DOM，所以要靠 input 事件同步状态。
    el.dispatchEvent(new InputEvent('input', { bubbles: true }));
    // 旧的选区节点已被替换，光标移到文首。
    el.focus();
    const start = document.createRange();
    start.selectNodeContents(el);
    start.collapse(true);
    selectionRef.current = start.cloneRange();
    const sel = window.getSelection();
    sel?.removeAllRanges();
    sel?.addRange(start);
    message.success(
      intlRef.current.formatMessage({ id: 'component.simpleEditor.cleaned' }),
    );
  }, []);

  // createButton returns an inline component, so calling it during render gives
  // the toolbar a new type on every keystroke and React remounts it.
  const [BtnAttachment, BtnAi, BtnCleanHtml] = useMemo(
    () => [
      createButton('Media', MEDIA_ICON, openAttachment),
      createButton('AI Assistant', 'AI', openAiPanel),
      createButton(
        intlRef.current.formatMessage({
          id: 'component.simpleEditor.clean-html-btn',
        }),
        '🧹',
        cleanEditorHtml,
      ),
    ],
    [openAttachment, openAiPanel, cleanEditorHtml],
  );

  const setInnerContent = useCallback(
    (html: string) => {
      props.setContent(html);
    },
    [props.setContent],
  );

  useImperativeHandle(ref, () => ({
    setInnerContent: setInnerContent,
  }));

  const uploadFile = async (file: File): Promise<any | null> => {
    const size = file.size;
    const md5Value = await calculateFileMd5(file);
    const chunkSize = 2 * 1024 * 1024; // 每个分片大小 2MB
    const totalChunks = Math.ceil(size / chunkSize);

    let hide = message.loading(
      intl.formatMessage({ id: 'component.editor.inserting' }),
      0,
    );
    let res: any;
    if (totalChunks > 1) {
      // 大于 chunkSize 的，使用分片上传
      let formData = new FormData();
      formData.append('file_name', file.name);
      formData.append('md5', md5Value as string);
      formData.append('chunks', totalChunks + '');
      for (let i = 0; i < totalChunks; i++) {
        const chunk = file.slice(i * chunkSize, (i + 1) * chunkSize);
        formData.set('chunk', i + '');
        formData.set('file', chunk, file.name);
        res = await uploadAttachment(formData);
        if (res.code !== 0) {
          hide();
          message.info(res.msg);
          return null;
        }
        hide();
        hide = message.loading(
          intl.formatMessage({ id: 'component.editor.inserting' }) +
            ' - ' +
            Math.ceil(((i + 1) * 100) / totalChunks) +
            '%',
          0,
        );
        if (res.data) {
          // 上传完成
          hide();
          return res.data;
        }
      }
      hide();
    } else {
      // 小于 chunkSize 的，直接上传
      let formData = new FormData();
      formData.append('file_name', file.name);
      formData.append('file', file);
      res = await uploadAttachment(formData);
      hide();
      if (res.code !== 0) {
        message.info(res.msg);
        return null;
      }
      return res.data;
    }
    return null;
  };

  const handleUpload = async (files: File[]) => {
    const result: any[] = [];
    for (const file of files) {
      const uploaded = await uploadFile(file);
      if (uploaded) {
        result.push(uploaded);
      }
    }
    return result;
  };

  const handleRangeFromPoint = (x: number, y: number) => {
    const el = editorRef.current;
    const doc: any = document;
    let range: Range | null = null;
    if (doc.caretRangeFromPoint) {
      range = doc.caretRangeFromPoint(x, y);
    } else if (doc.caretPositionFromPoint) {
      const point = doc.caretPositionFromPoint(x, y);
      if (point) {
        range = document.createRange();
        range.setStart(point.offsetNode, point.offset);
        range.collapse(true);
      }
    }
    return el && range && el.contains(range.commonAncestorContainer)
      ? range
      : null;
  };

  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    if (!Array.from(e.dataTransfer.types).includes('Files')) {
      return;
    }
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
    setDragOver(true);
  };

  const handleDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
    if (!e.currentTarget.contains(e.relatedTarget as Node)) {
      setDragOver(false);
    }
  };

  const handleDrop = async (e: React.DragEvent<HTMLDivElement>) => {
    const el = editorRef.current;
    const files = Array.from(e.dataTransfer.files);
    if (files.length === 0 || !el || !el.isConnected) {
      return;
    }
    e.preventDefault();
    setDragOver(false);
    // Read the drop position now: the event is gone once the upload finishes.
    const dropRange = handleRangeFromPoint(e.clientX, e.clientY);
    const items = await handleUpload(files);
    if (items.length === 0) {
      return;
    }
    selectionRef.current = dropRange;
    insertBlockHtml(items.map(buildEmbedHtml).join(''));
  };

  const handleGetSelection = useCallback(
    () => (selectionRef.current ? rangeToMarkdown(selectionRef.current) : ''),
    [],
  );

  const handleAppendBlock = useCallback(
    (_: string, htmlText: string) => {
      selectionRef.current = null;
      // The AI answer is already markdown converted to block-level HTML.
      insertBlockHtml(htmlText);
    },
    [insertBlockHtml],
  );

  const handleReplaceSelection = useCallback(
    (_: string, htmlText: string) => {
      insertBlockHtml(htmlText);
    },
    [insertBlockHtml],
  );

  const handleSetContent = useCallback(
    (_: string, htmlText: string) => {
      props.setContent(htmlText);
    },
    [props.setContent],
  );

  const handleGetContent = useCallback(() => {
    return htmlToMarkdown(props.content || '');
  }, [props.content]);

  return (
    <div
      className={
        'editor-container ' + props.className + (dragOver ? ' drag-over' : '')
      }
      style={{
        marginTop: '10px',
      }}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <Editor
        ref={editorRef}
        value={props.content}
        onChange={(e) => {
          props.setContent(e.target.value);
        }}
      >
        <Toolbar>
          <BtnUndo />
          <BtnRedo />
          <Separator />
          <BtnStyles />
          <Separator />
          <BtnBold />
          <BtnItalic />
          <BtnUnderline />
          <BtnStrikeThrough />
          <Separator />
          <BtnNumberedList />
          <BtnBulletList />
          <Separator />
          <BtnLink />
          <BtnClearFormatting />
          <HtmlButton />
          <Separator />
          <BtnAttachment />
          <BtnAi />
          <BtnCleanHtml />
        </Toolbar>
      </Editor>
      <AiEditorOverlay
        visible={aiVisible}
        onClose={() => setAiVisible(false)}
        getSelection={handleGetSelection}
        appendBlock={handleAppendBlock}
        replaceSelection={handleReplaceSelection}
        setContent={handleSetContent}
        getContent={handleGetContent}
      />
    </div>
  );
});

export default SimpleEditor;
