import { IDomEditor, IEditorConfig } from '@wangeditor/editor';
import '@wangeditor/editor/dist/css/style.css';
import { Editor, Toolbar } from '@wangeditor/editor-for-react';
import { useEffect, useState } from 'react';
import config from '@/services/config';
import { getStore } from '@/utils/store';
import { Boot } from '@wangeditor/editor';
import {MaterialsMenuConf} from './material/menu';
import {HtmlMenuConf} from './html/menu';
import AttachmentSelect from '../attachment';
import MonacoEditor from 'react-monaco-editor';
import './index.less';
import { message } from 'antd';

// 注册。要在创建编辑器之前注册，且只能注册一次，不可重复注册。
Boot.registerMenu(MaterialsMenuConf);
Boot.registerMenu(HtmlMenuConf);

export type WangEditorProps = {
  className: string;
  content: string;
  setContent: (html: any) => Promise<void>;
};

var editorInsertFn: (url: string, alt?: string, href?: string) => void;
var code = '';

const WangEditor: React.FC<WangEditorProps> = (props) => {
  const [editor, setEditor] = useState<IDomEditor | null>(null);
  const [attachVisible, setAttachVisible] = useState<boolean>(false);
  const [htmlMode, setHtmlMode] = useState<boolean>(false);

  const editorConfig: Partial<IEditorConfig> = {};
  editorConfig.placeholder = '请输入内容...';
  editorConfig.MENU_CONF = {};
  editorConfig.MENU_CONF['uploadImage'] = {
    customBrowseAndUpload(insertFn: any) {
      editorInsertFn = insertFn;
      setAttachVisible(true);
    }
  };
  editorConfig.MENU_CONF['uploadVideo'] = {
    server: config.baseUrl + '/attachment/upload',
    allowedFileTypes: ['video/mp4'],
    headers: {
      admin: getStore('adminToken'),
    },
    customInsert(res: any, insertFn: any) {
      res = res.data || {};
      if (res.code !== 0 ){
        message.info(res.msg);
      } else {
        message.info(res.msg || '上传成功');
        insertFn(res.logo);
      }
    },
    fieldName: 'file',
  };
  editorConfig.MENU_CONF['html'] = {
    setMode(mode: boolean) {
      setHtmlMode(mode)
      if(!mode && editorInsertFn) {
        editorInsertFn(code)
      }
    },
    customHtml(insertFn: any) {
      editorInsertFn = insertFn;
    }
  };
  const handleSelectImages = (e: any) => {
    if (editorInsertFn) {
      editorInsertFn(e.logo, e.file_name);
    }
    setAttachVisible(false);
  }

  const onChangeCode = (newCode: string) => {
    if (code != newCode) {
      code = newCode
    }
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
      keys: ['html','material'],
    },
  };

  const content =
    props.content?.length > 0 && props.content[0] === '<'
      ? props.content
      : '<p>' + (props.content || '') + '</p>';

  return (
    <div
      className={'editor-container ' + props.className}
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
        onChange={(editor) => {let html = editor.getHtml();props.setContent(html);code = html}}
        style={{ height: '500px' }}
      />
      <div style={{display: htmlMode ? 'block' : 'none'}} className='tmp-editor'>
      {htmlMode && <MonacoEditor
              height={523}
              language={'html'}
              theme="vs-dark"
              value={code}
              options={{
                selectOnLineNumbers: false,
                wordWrap: 'on'
              }}
              onChange={onChangeCode}
              editorDidMount={() => {}}
            />}
      </div>
      <AttachmentSelect onSelect={(row) => {handleSelectImages(row)}} onCancel={(flag) => {setAttachVisible(flag)}} visible={attachVisible} manual={true} />
    </div>
  );
};

export default WangEditor;
