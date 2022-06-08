import { IDomEditor, IEditorConfig } from '@wangeditor/editor';
import '@wangeditor/editor/dist/css/style.css';
import { Editor, Toolbar } from '@wangeditor/editor-for-react';
import { useEffect, useState } from 'react';
import config from '@/services/config';
import { getStore } from '@/utils/store';
import { Boot } from '@wangeditor/editor';
import {MaterialsMenuConf} from './material/menu';
import {ImagesMenuConf} from './images/menu';

// 注册。要在创建编辑器之前注册，且只能注册一次，不可重复注册。
Boot.registerMenu(MaterialsMenuConf);
Boot.registerMenu(ImagesMenuConf);

export type WangEditorProps = {
  className: string;
  content: string;
  setContent: (html: any) => Promise<void>;
};

const WangEditor: React.FC<WangEditorProps> = (props) => {
  const [editor, setEditor] = useState<IDomEditor | null>(null);
  const editorConfig: Partial<IEditorConfig> = {};
  editorConfig.placeholder = '请输入内容...';
  editorConfig.MENU_CONF = {};
  editorConfig.MENU_CONF['uploadImage'] = {
    server: config.baseUrl + '/attachment/upload',
    allowedFileTypes: ['image/*'],
    headers: {
      admin: getStore('adminToken'),
    },
    customInsert(res: any, insertFn: any) {
      res = res.data || {};
      insertFn(res.src, res.title, null);
    },
    fieldName: 'file',
  };
  editorConfig.MENU_CONF['uploadVideo'] = {
    server: config.baseUrl + '/attachment/upload',
    allowedFileTypes: ['video/mp4'],
    headers: {
      admin: getStore('adminToken'),
    },
    customInsert(res: any, insertFn: any) {
      res = res.data || {};
      insertFn(res.src);
    },
    fieldName: 'file',
  };

  //const defaultContent = [{ type: 'paragraph', children: [{ text: '' }] }];

  useEffect(() => {
    return () => {
      if (editor == null) return;
      editor.destroy();
      setEditor(null);
    };
  }, [editor]);

  // ----------------------- toolbar config -----------------------
  const toolbarConfig = {
    // 可配置 toolbarKeys: [...]
    insertKeys: {
      index: 0, // 自定义插入的位置
      keys: ['material', 'attachment'],
    },
  };

  const content =
    props.content?.length > 0 && props.content[0] === '<'
      ? props.content
      : '<p>' + (props.content || '') + '</p>';

  return (
    <div
      className={props.className}
      data-testid="editor-container"
      style={{ border: '1px solid #ccc', marginTop: '10px' }}
    >
      <Toolbar
        editor={editor}
        defaultConfig={toolbarConfig}
        style={{ borderBottom: '1px solid #ccc' }}
      />

      {/* 渲染 editor */}
      <Editor
        defaultHtml={content}
        defaultConfig={editorConfig}
        //defaultContent={defaultContent}
        mode="default"
        onCreated={(editor) => {
          setEditor(editor);
        }}
        onChange={(editor) => props.setContent(editor.getHtml())}
        style={{ height: '500px' }}
      />
      <div style={{display: 'none'}} id='tmp-editor'></div>
    </div>
  );
};

export default WangEditor;
