import ProList from '@ant-design/pro-list';
import { pluginGetMaterialCategories, pluginGetMaterials } from '@/services/plugin/material';
import { Button, Modal, Space, Tag } from 'antd';
import './index.less';
import { FileSearchOutlined, PlusSquareOutlined } from '@ant-design/icons';
import { DomEditor, IDomEditor, IButtonMenu } from '@wangeditor/core';

import { Text } from 'slate';
import { SlateElement } from '@wangeditor/editor';

//【注意】需要把自定义的 Element 引入到最外层的 custom-types.d.ts

export type MaterialElement = {
  type: 'material';
  id: string;
  title: string;
  content: string;
  children: Text[];
};


// 定义菜单 class
class MaterialMenu implements IButtonMenu {
  readonly title = '插入内容素材';
  readonly iconSvg =
    '<svg viewBox="64 64 896 896" focusable="false" data-icon="reconciliation" width="1em" height="1em" fill="currentColor" aria-hidden="true"><path d="M676 623c-18.8 0-34 15.2-34 34s15.2 34 34 34 34-15.2 34-34-15.2-34-34-34zm204-455H668c0-30.9-25.1-56-56-56h-80c-30.9 0-56 25.1-56 56H264c-17.7 0-32 14.3-32 32v200h-88c-17.7 0-32 14.3-32 32v448c0 17.7 14.3 32 32 32h336c17.7 0 32-14.3 32-32v-16h368c17.7 0 32-14.3 32-32V200c0-17.7-14.3-32-32-32zM448 848H176V616h272v232zm0-296H176v-88h272v88zm20-272v-48h72v-56h64v56h72v48H468zm180 168v56c0 4.4-3.6 8-8 8h-48c-4.4 0-8-3.6-8-8v-56c0-4.4 3.6-8 8-8h48c4.4 0 8 3.6 8 8zm28 301c-50.8 0-92-41.2-92-92s41.2-92 92-92 92 41.2 92 92-41.2 92-92 92zm92-245c0 4.4-3.6 8-8 8h-48c-4.4 0-8-3.6-8-8v-96c0-4.4 3.6-8 8-8h48c4.4 0 8 3.6 8 8v96zm-92 61c-50.8 0-92 41.2-92 92s41.2 92 92 92 92-41.2 92-92-41.2-92-92-92zm0 126c-18.8 0-34-15.2-34-34s15.2-34 34-34 34 15.2 34 34-15.2 34-34 34z"  fill="#595959"></path></svg>';

  readonly tag = 'button';
  getValue(editor: IDomEditor): string | boolean {
    // 插入菜单，不需要 value
    return '';
  }
  isActive(editor: IDomEditor): boolean {
    // 任何时候，都不用激活 menu
    return false;
  }
  async exec(editor: IDomEditor, value: string | boolean) {
    const previewDetail = (detail: any) => {
      Modal.info({
        title: detail.title,
        icon: '',
        width: 600,
        content: (
          <div
            dangerouslySetInnerHTML={{ __html: detail.content }}
          ></div>
        ),
      });
    };

    const useDetail = (detail: any) => {
      editor.restoreSelection();

      // 插入节点
      const materialElem: MaterialElement = {
        type: 'material',
        id: detail.id,
        title: detail.title,
        content: detail.content,
        children: [{ text: '' }],
      };
      const content = MaterialToHtml(materialElem, "");
      editor.insertBreak();
      editor.dangerouslyInsertHtml(content);
      editor.insertBreak();
      materialModal.destroy();
    };

    let res = await pluginGetMaterialCategories();
    let categories = (res.data || []).reduce((pre: Object, cur: any) => {
      pre[cur.id] = cur.title;
      return pre;
    }, []);

    const materialModal = Modal.confirm({
      width: 800,
      icon: '',
      title: false,
      okButtonProps: {
        className: 'hidden',
      },
      className: 'material-modal',
      onOk: () => {},
      content: (
        <ProList<any>
          className='material-table'
          rowKey="id"
          request={(params) => {
            return pluginGetMaterials(params);
          }}
          pagination={{
            defaultPageSize: 6,
          }}
          showActions="hover"
          showExtra="hover"
          search={{
            span: 12,
            labelWidth: 120,
          }}
          metas={{
            title: {
              search: false,
              dataIndex: 'title',
            },
            description: {
              dataIndex: 'content',
              search: false,
              render: (text: any) => {
                return (
                  <div
                    className="material-description"
                    dangerouslySetInnerHTML={{ __html: text }}
                  ></div>
                );
              },
            },
            subTitle: {
              search: false,
              render: (text: any, row: any) => {
                return (
                  <Space size={0}>
                    <Tag
                      className="link"
                      onClick={() => {
                        previewDetail(row);
                      }}
                    >
                      <FileSearchOutlined /> 预览
                    </Tag>
                    <Tag
                      color="blue"
                      className="link"
                      onClick={() => {
                        useDetail(row);
                      }}
                    >
                      <PlusSquareOutlined /> 使用
                    </Tag>
                  </Space>
                );
              },
            },
            category_id: {
              title: '分类筛选',
              valueType: 'select',
              valueEnum: categories,
            },
          }}
        />
      ),
    });
  }
  isDisabled(editor: IDomEditor): boolean {
    return isMenuDisabled(editor);
  }
}

// 定义菜单配置
export const MaterialsMenuConf = {
  key: 'material', // menu key ，唯一。注册之后，可配置到工具栏
  factory() {
    return new MaterialMenu();
  },
};

function isMenuDisabled(editor: IDomEditor): boolean {
  if (editor.selection == null) return true;

  const selectedElems = DomEditor.getSelectedElems(editor);
  const notMatch = selectedElems.some((elem) => {
    if (editor.isVoid(elem)) return true;
  });
  if (notMatch) return true; // disabled
  return false; // enable
}

// 生成 html 的函数
function MaterialToHtml(elem: SlateElement, childrenHtml: string): string {
  const { title = '', id = '', content = '' } = elem as MaterialElement;
  const html = `<div data-w-e-type="material" data-w-e-is-void data-material="${id}" data-title="${title}">${content}</div>`;
  return html;
}
