import {
  AiEditor,
  AiService,
  AiServiceConfig,
  Editor,
  UploadResult,
  type AiEditorContext,
  type AiGenerateRequest,
} from 'aieditor';
import 'aieditor/style.css';

import { uploadAttachment } from '@/services';
import config from '@/services/config';
import { calculateFileMd5 } from '@/utils';
import { getSessionStore, getStore } from '@/utils/store';
import { ArrowLeftOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Button, Input, message, Modal, Space, Spin, Tooltip } from 'antd';
import { Code, Heading2, Heading3, Images, Layers, Video } from 'lucide';
import {
  forwardRef,
  lazy,
  Suspense,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react';
import AttachmentSelect from '../attachment';
import './index.less';
import MaterialSelect, { MaterialElement } from './material';

const MonacoEditor = lazy(() => import('react-monaco-editor'));

export type NewAiEditorProps = {
  className: string;
  content: string;
  field: string;
  setContent: (html: any) => Promise<void>;
  ref: any;
};

let codes: any = {};

const NewAiEditor: React.FC<NewAiEditorProps> = forwardRef((props, ref) => {
  const divRef = useRef<HTMLDivElement>(null);
  const aiEditorRef = useRef<AiEditor | null>(null);
  const [videoVisible, setVideoVisible] = useState(false);
  const [curEditor, setCurEditor] = useState<Editor | null>(null);
  const [htmlMode, setHtmlMode] = useState<boolean>(false);
  const [videoUrl, setVideoUrl] = useState<string>('');
  const [materialVisible, setMaterialVisible] = useState(false);
  const [attachVisible, setAttachVisible] = useState(false);
  const intl = useIntl();
  function setInnerContent(content: string) {
    // 判断是不是Markdown
    aiEditorRef.current?.setContent(content);
  }

  useImperativeHandle(ref, () => ({
    setInnerContent: setInnerContent,
  }));

  const getImagehtml = (item: any) => {
    if (
      item.is_image === 1 ||
      item.file_location.indexOf('.webp') !== -1 ||
      item.file_location.indexOf('.bmp') !== -1 ||
      item.file_location.indexOf('.png') !== -1 ||
      item.file_location.indexOf('.gif') !== -1 ||
      item.file_location.indexOf('.jpg') !== -1 ||
      item.file_location.indexOf('.jpeg') !== -1 ||
      item.file_location.indexOf('.svg') !== -1
    ) {
      // img
      return `<img src="${item.file_path}" alt="${item.file_name}" title="${item.file_name}" />`;
      //return `![${item.file_name}](${item.file_path})`;
    } else if (
      item.is_image === 2 ||
      item.file_location.indexOf('.mp4') !== -1 ||
      item.file_location.indexOf('.ogg') !== -1 ||
      item.file_location.indexOf('.webm') !== -1
    ) {
      return `<video controls="controls" controlslist="nodownload" poster="${
        item.logo
      }"><source src="${
        item.file_path
      }" type="video/${item.file_location.substr(
        item.file_location.lastIndexOf('.') + 1,
      )}">Your browser does not support video tags.</video>`;
    } else if (
      item.file_location.indexOf('.mp3') !== -1 ||
      item.file_location.indexOf('.wav') !== -1
    ) {
      return `<audio src="${item.file_path}" controls="controls"><source src="${
        item.file_path
      }" type="audio/${item.file_location.substr(
        item.file_location.lastIndexOf('.') + 1,
      )}">Your browser does not support audio tags.</audio>`;
    } else if (item.is_image === 3) {
      return `<iframe src="${item.file_location}" title="${item.file_name}" width="100%" height="480px" frameborder="0" allowfullscreen="true"></iframe>`;
    } else {
      return `<a href="${item.file_path}" target="_blank" title="${item.file_name}">${item.file_name}</a>`;
    }
  };
  function attachPlugin(editor: Editor) {
    setCurEditor(editor);
    setAttachVisible(true);
  }

  function attachVideo(editor: Editor) {
    setCurEditor(editor);
    setVideoVisible(true);
  }

  // 从 iframe 嵌入代码或裸链接中提取 YouTube 视频信息
  const parseYouTubeInput = (input: string) => {
    const trimmed = input.trim();
    let src = trimmed;
    let width: number | undefined;
    let height: number | undefined;
    const iframeMatch = trimmed.match(/<iframe[^>]*>/i);
    if (iframeMatch) {
      const tag = iframeMatch[0];
      const srcMatch = tag.match(/src\s*=\s*["']([^"']+)["']/i);
      if (!srcMatch) return null;
      src = srcMatch[1];
      const w = tag.match(/width\s*=\s*["']?(\d+)/i);
      const h = tag.match(/height\s*=\s*["']?(\d+)/i);
      width = w ? Number(w[1]) : undefined;
      height = h ? Number(h[1]) : undefined;
    }
    // 校验是 YouTube 链接（watch / youtu.be / embed / shorts / nocookie）
    if (
      !/youtube(-nocookie)?\.com\/(watch\?v=|embed\/|shorts\/)/i.test(src) &&
      !/^https?:\/\/youtu\.be\//i.test(src)
    ) {
      return null;
    }
    return { src, width, height };
  };

  const handleSubmitVideo = () => {
    if (!videoUrl) {
      message.error(
        intl.formatMessage({ id: 'component.editor.video.required' }),
      );
      return;
    }
    if (curEditor) {
      const trimmed = videoUrl.trim();
      // YouTube 链接或 iframe 嵌入代码：走 youtube 节点，保持原链接、可选中编辑/删除
      const youtube = parseYouTubeInput(trimmed);
      if (youtube) {
        curEditor
          .chain()
          .focus()
          .setYoutubeVideo({
            src: youtube.src,
            width: youtube.width,
            height: youtube.height,
          } as any)
          .run();
      } else if (trimmed.indexOf('<') === 0) {
        // 其它 iframe 嵌入代码原样插入
        curEditor.chain().focus().insertContent(trimmed).run();
      } else if (trimmed.indexOf('http') === 0) {
        const videoHtml = `<video controls="controls" controlslist="nodownload" poster=""><source src="${trimmed}" type="video/mp4">Your browser does not support video tags.</video>`;
        curEditor.chain().focus().insertContent(videoHtml).run();
      } else {
        curEditor.chain().focus().insertContent(trimmed).run();
      }
    }
    setVideoUrl('');
    setVideoVisible(false);
  };

  const handleSelectVideo = () => {
    if (curEditor) {
      attachPlugin(curEditor);
    }
  };

  const handleSelectAttachment = (rows: any[]) => {
    let addon = [];
    for (let i = 0; i < rows.length; i++) {
      addon.push(getImagehtml(rows[i]));
    }
    // if (!curEditor?.isFocused()) {
    //   curEditor?.focus();
    // }
    curEditor?.chain().focus().insertContent(addon.join('\n')).run();
    setAttachVisible(false);
    setVideoVisible(false);
  };

  const showMaterial = (editor: Editor) => {
    setCurEditor(editor);
    setMaterialVisible(true);
  };

  const handleSelectMaterial = (row: MaterialElement) => {
    if (curEditor) {
      const insertData = `<div data-w-e-type="material" data-w-e-is-void data-material="${row.id}" data-title="${row.title}">${row.content}</div>`;

      curEditor?.chain().focus().insertContent(insertData).run();
      setMaterialVisible(false);
    }
  };

  // export declare interface UploadResult {
  //     url: string;
  //     alt?: string;
  //     title?: string;
  //     poster?: string;
  //     name?: string;
  //     mimeType?: string;
  //     size?: number;
  // }

  const handleUpload = async (file: File): Promise<UploadResult> => {
    const result: any = {
      errorCode: 0,
      data: {},
    };
    const size = file.size;
    const md5Value = await calculateFileMd5(file);
    const chunkSize = 2 * 1024 * 1024; // 每个分片大小 2MB
    const totalChunks = Math.ceil(size / chunkSize);

    let hide = message.loading({
      key: 'uploading',
      content: intl.formatMessage({ id: 'component.footer.submitting' }),
      duration: 0,
    });

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
          result.errorCode = res.code;
          result.msg = res.msg;
          return Promise.reject(result);
        }
        hide();
        hide = message.loading({
          key: 'uploading',
          content:
            intl.formatMessage({ id: 'component.footer.submitting' }) +
            ' - ' +
            Math.ceil(((i + 1) * 100) / totalChunks) +
            '%',
          duration: 0,
        });
        if (res.data) {
          // 上传完成
          hide();
          result.url = res.data.file_path;
          result.alt = res.data.file_name;
          result.align = 'center';
          result.width = '100%';
          result.height = 'auto';
          return result;
        }
      }
      hide();
    } else {
      // 小于 chunkSize 的，直接上传
      let formData = new FormData();
      formData.append('file', file);
      res = await uploadAttachment(formData);
      hide();
      if (res.code !== 0) {
        message.info(res.msg);
        result.errorCode = res.code;
        result.msg = res.msg;
        return Promise.reject(result);
      } else {
        result.url = res.data.file_path;
        result.alt = res.data.file_name;
        result.align = 'center';
        result.width = '100%';
        result.height = 'auto';
      }
    }

    return result;
  };

  const showSourceCode = (editor: Editor) => {
    setCurEditor(editor);
    let htmlCode = editor.getHTML();
    // 移除 a标签的 rel 属性，其它属性保留
    htmlCode = htmlCode.replace(/<a(\s[^>]*)?>/gi, (match) => {
      return match.replaceAll(/\srel\s*=\s*["'][^"']*["']/gi, '');
    });
    codes[props.field] = htmlCode;
    setHtmlMode(true);
  };

  const hideSourceCode = () => {
    aiEditorRef.current?.setContent(codes[props.field]);
    setHtmlMode(false);
  };

  const onChangeCode = (newCode: string) => {
    if (codes[props.field] !== newCode) {
      codes[props.field] = newCode;
      props.setContent(codes[props.field]);
    }
  };

  const initEditor = async () => {
    if (!divRef.current) return;
    // 官方默认
    let aiCfg: AiService | AiServiceConfig = {
      provider: 'custom',
      model: 'custom',
      baseURL: '/api/openai/v1',
      dangerouslyAllowBrowser: true,
      timeout: 30_000,
      maxRetries: 1,
      async generate(request: AiGenerateRequest, context: AiEditorContext) {
        // 组装请求
        const scope =
          request.scope ?? (context.selectedText ? 'selection' : 'document');
        const conversation = request.history?.length
          ? `${request.history
              .map(
                (message) =>
                  `${message.role === 'user' ? 'User' : 'Assistant'}: ${
                    message.content
                  }`,
              )
              .join('\n')}\nUser: ${request.prompt}`
          : request.prompt;
        if (scope === 'none') return conversation;

        // 这里发送纯文本而非 HTML，避免标记噪声干扰常规写作请求。
        const content =
          scope === 'selection' ? context.selectedText : context.text;
        const label = scope === 'selection' ? 'Selected text' : 'Document';
        const prompt = `${conversation}\n\n${label}:\n${content}`;

        let adminToken =
          getSessionStore('adminToken') || getStore('adminToken');
        const response = await fetch(config.baseUrl + '/anqi/ai/chat', {
          method: 'POST',
          headers: {
            admin: adminToken,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            prompt: prompt,
            instructions: request.instructions,
          }),
          signal: request.signal,
        });

        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        // 检查响应类型：如果返回的是 JSON（非 SSE），说明需要特殊处理
        const contentType = response.headers.get('Content-Type') || '';
        if (contentType.includes('application/json')) {
          const jsonResp = await response.json();
          throw new Error(
            jsonResp.msg ||
              intl.formatMessage({ id: 'component.ai.request-failed' }),
          );
        }
        const reader = response.body?.getReader();
        if (!reader) {
          throw new Error('No readable stream');
        }
        const decoder = new TextDecoder();
        let buffer = '';
        let tmpContent = '';

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const events = buffer.split('\n\n');
          buffer = events.pop() || '';

          for (const event of events) {
            if (!event.trim()) continue;

            const lines = event.split('\n');
            let eventType = 'message';
            let data = '';

            for (const line of lines) {
              if (line.startsWith('event:')) {
                eventType = line.substring(6).trim();
              } else if (line.startsWith('data:')) {
                data = line.substring(5).trim();
              }
            }

            // end 事件：AI 响应结束
            if (eventType === 'end') {
              request.onChunk?.('');
              continue;
            }

            // message 事件：AI 响应内容
            if (eventType === 'message') {
              try {
                const parsed = JSON.parse(data || '{}');
                if (parsed.v !== undefined) {
                  tmpContent += parsed.v;
                  request.onChunk?.(parsed.v);
                }
              } catch (e) {
                console.error('Error parsing SSE data:', e);
              }
              continue;
            }

            // warning 事件：显示警告
            if (eventType === 'warning') {
              try {
                const parsed = JSON.parse(data || '{}');
                if (parsed.v) {
                  message.warning(parsed.v);
                }
              } catch (e) {
                // ignore
              }
              continue;
            }

            // config 事件：AI 配置错误
            if (eventType === 'config') {
              message.error(
                intl.formatMessage({ id: 'component.ai.config-error' }),
              );
              continue;
            }
          }
        }

        if (request.stream || request.onChunk) {
          return tmpContent;
        }

        return { text: tmpContent, raw: response };
      },
    };

    if (!aiEditorRef.current) {
      const aiEditor = new AiEditor({
        element: divRef.current,
        placeholder: intl.formatMessage({ id: 'component.editor.placeholder' }),
        content: props.content,
        // 关闭 UniqueID 扩展：它默认给每个块级节点（段落/标题/表格/图片等）
        // 生成 UUID，并在渲染时统一写成 data-id="..."。这些 ID 只服务于
        // 协作编辑的块定位，本站是单机 CMS，用不上；而正文里的
        // <p data-id="..."> 会随文章一起存进库、进模板、进 AI 抓取结果，
        // 徒增体积且没有任何语义。
        // 关闭后扩展不注册，schema 里没有该属性：解析时不读、渲染时不输出，
        // 历史内容里的 data-id 会在下次打开保存时自然消失。
        // ⚠️ 标题的 id / data-toc-id 属于 TableOfContents 扩展（另一套），
        // 不受影响，文档目录功能保持正常。
        uniqueId: false,
        // aieditor 内部给 youtube 扩展硬编码了 nocookie: true，会把
        // youtube.com/watch 链接改写成 youtube-nocookie.com/embed，导致
        // 前台无法播放。tiptap youtube 扩展在渲染时运行时读取该选项，
        // 这里在创建前改回 false，保持用户原始链接域名。
        onBeforeCreate: (editor: any) => {
          const youtube = editor.extensionManager?.extensions?.find(
            (ext: any) => ext.name === 'youtube',
          );
          if (youtube) {
            youtube.options.nocookie = false;
          }
        },
        toolbar: {
          menus: (defaults) => [
            ...defaults,
            {
              type: 'button',
              key: 'insert-video',
              label: intl.formatMessage({
                id: 'component.newAiEditor.toolbar.insert-video',
              }),
              icon: Video,
              onClick: ({ editor }) => attachVideo(editor),
              tip: 'video',
              isEnabled: ({ editor }) => editor.isEditable,
            },
            {
              type: 'button',
              key: 'images',
              label: intl.formatMessage({
                id: 'component.newAiEditor.toolbar.image-attachment',
              }),
              icon: Images,
              onClick: ({ editor }) => attachPlugin(editor),
              tip: 'Images',
              isEnabled: ({ editor }) => editor.isEditable,
            },
            {
              type: 'button',
              key: 'material',
              label: intl.formatMessage({
                id: 'component.newAiEditor.toolbar.content-material',
              }),
              icon: Layers,
              onClick: ({ editor }) => showMaterial(editor),
              tip: 'material',
              isEnabled: ({ editor }) => editor.isEditable,
            },
            {
              type: 'button',
              key: 'source-code',
              label: intl.formatMessage({
                id: 'component.newAiEditor.toolbar.source-code',
              }),
              icon: Code,
              onClick: ({ editor }) => showSourceCode(editor),
              tip: 'source-code',
            },
          ],
        },
        translations: {
          zh: {
            material: '内容片段',
          },
          en: {
            material: 'Content snippets',
          },
        },
        ai: aiCfg,
        aiChat: {
          welcomeMessage: intl.formatMessage({
            id: 'component.newAiEditor.chat.welcome',
          }),
          placeholder: intl.formatMessage({
            id: 'component.newAiEditor.chat.placeholder',
          }),
          toolApproval: 'always',
        },
        onUpdate: (ed: Editor) => {
          let htmlCode = ed.getHTML();
          // 移除 a标签的 rel 属性，其它属性保留
          htmlCode = htmlCode.replace(/<a(\s[^>]*)?>/gi, (match) => {
            return match.replaceAll(/\srel\s*=\s*["'][^"']*["']/gi, '');
          });
          props.setContent(htmlCode);
          codes[props.field] = htmlCode;
        },
        uploader: {
          accept: {
            image: 'image/*',
            audio: 'audio/mpeg,audio/wav,audio/ogg',
            video: 'video/mp4,video/webm',
            attachment: '.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.zip',
          },
          maxSize: {
            image: 20 * 1024 * 1024,
            audio: 50 * 1024 * 1024,
            video: 2000 * 1024 * 1024,
            attachment: 200 * 1024 * 1024,
          },
          async upload(file) {
            return handleUpload(file);
          },
        },
        sidebar: {
          defaultItem: false,
        },
        bubbleMenu: {
          items: [
            'bubble-ai',
            {
              type: 'button',
              key: 'h2',
              label: 'h2',
              icon: Heading2,
              onClick: ({ editor }) => {
                if (editor.isActive('heading', { level: 2 })) {
                  return editor.chain().setParagraph().focus().run();
                }
                return editor
                  .chain()
                  .setHeading({ level: 2 as any })
                  .focus()
                  .run();
              },
              isActive: ({ editor }) => {
                return editor.isActive('heading', { level: 2 });
              },
              isEnabled: ({ editor }) => !editor.state.selection.empty,
            },

            {
              type: 'button',
              key: 'h3',
              label: 'h3',
              icon: Heading3,
              onClick: ({ editor }) => {
                if (editor.isActive('heading', { level: 3 })) {
                  return editor.chain().setParagraph().focus().run();
                }
                return editor
                  .chain()
                  .setHeading({ level: 3 as any })
                  .focus()
                  .run();
              },
              isActive: ({ editor }) => {
                return editor.isActive('heading', { level: 3 });
              },
              isEnabled: ({ editor }) => !editor.state.selection.empty,
            },
            'bold',
            'italic',
            'underline',
            'strike',
            'code',
            'bubble-link',
            'heading',
            'clear-formatting',
          ],
        },
      });

      aiEditorRef.current = aiEditor;
    }
  };

  useEffect(() => {
    if (!divRef.current) return;

    initEditor();

    return () => {
      if (aiEditorRef.current) {
        aiEditorRef.current.destroy();
        aiEditorRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (ref) {
      if (typeof ref === 'function') {
        ref(divRef.current);
      } else {
        ref.current = divRef.current;
      }
    }
  }, [ref]);

  useEffect(() => {
    if (
      aiEditorRef.current &&
      props.content !== aiEditorRef.current.getHTML()
    ) {
      //  aiEditorRef.current.setContent(props.content || "");
    }
  }, [props.content]);

  return (
    <div
      className={'editor-container ' + props.className}
      style={{ marginTop: '10px' }}
    >
      <div ref={divRef} {...props} style={{ height: '100%' }} />
      {videoVisible && (
        <Modal
          title={intl.formatMessage({ id: 'component.editor.video.title' })}
          open={videoVisible}
          onCancel={() => setVideoVisible(false)}
          width={600}
          footer={null}
        >
          <div className="mb-normal">
            <Input.TextArea
              placeholder={intl.formatMessage({
                id: 'component.editor.video.required',
              })}
              value={videoUrl}
              onChange={(e) => setVideoUrl(e.target.value)}
            />
          </div>
          <Space
            align="center"
            style={{ width: '100%', justifyContent: 'center' }}
          >
            <Button type="primary" onClick={() => handleSubmitVideo()}>
              {intl.formatMessage({ id: 'component.editor.submit' })}
            </Button>
            <Button
              onClick={() => {
                setVideoVisible(false);
                handleSelectVideo();
              }}
            >
              {intl.formatMessage({ id: 'component.editor.video.select' })}
            </Button>
          </Space>
        </Modal>
      )}
      <div
        style={{ display: htmlMode ? 'block' : 'none' }}
        className="tmp-editor"
      >
        <div className="html-menus">
          <div className="menu-item" onClick={() => hideSourceCode()}>
            <Tooltip
              title={intl.formatMessage({
                id: 'component.editor.mode.return-view',
              })}
            >
              <ArrowLeftOutlined className="icon" />
            </Tooltip>
          </div>
        </div>
        {htmlMode && (
          <Suspense
            fallback={
              <Spin style={{ display: 'block', margin: '200px auto' }} />
            }
          >
            <MonacoEditor
              language={'html'}
              theme="vs-dark"
              value={codes[props.field]}
              options={{
                minimap: {
                  enabled: false,
                },
                selectOnLineNumbers: false,
                wordWrap: 'on',
              }}
              onChange={onChangeCode}
              editorDidMount={() => {}}
              editorWillUnmount={() => {}}
            />
          </Suspense>
        )}
      </div>
      {materialVisible && (
        <MaterialSelect
          open={materialVisible}
          onCancel={() => setMaterialVisible(false)}
          onSelect={handleSelectMaterial}
        />
      )}
      {attachVisible && (
        <AttachmentSelect
          open={attachVisible}
          onCancel={() => setAttachVisible(false)}
          onSelect={handleSelectAttachment}
          multiple={true}
          manual={true}
        />
      )}
    </div>
  );
});

export default NewAiEditor;
