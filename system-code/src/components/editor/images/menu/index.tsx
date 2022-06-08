import ProList from '@ant-design/pro-list';
import { getAttachmentCategories, getAttachments } from '@/services/attachment';
import { Avatar, Button, Modal, Space, Image } from 'antd';
import './index.less';
import { FileSearchOutlined, PlusSquareOutlined } from '@ant-design/icons';
import { DomEditor, IDomEditor, IButtonMenu } from '@wangeditor/core';

// 定义菜单 class
class ImageMenu implements IButtonMenu {
  readonly title = '插入图片素材';
  readonly iconSvg =
    '<svg t="1653724147741" class="icon" viewBox="0 0 1127 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="2178" width="64" height="64"><path d="M51.2 102.4l1024 0 0 51.2-1024 0 0-51.2Z" p-id="2179" fill="#595959"></path><path d="M0 211.4048l0 805.9904C0 1021.0304 3.2256 1024 7.2192 1024L1119.232 1024c3.9936 0 7.168-2.9696 7.168-6.6048L1126.4 211.4048C1126.4 207.7696 1123.1744 204.8 1119.232 204.8L7.2192 204.8C3.2256 204.8 0 207.7696 0 211.4048zM896 307.2C938.3936 307.2 972.8 341.6064 972.8 384 972.8 426.3936 938.3936 460.8 896 460.8S819.2 426.3936 819.2 384C819.2 341.6064 853.6064 307.2 896 307.2zM102.2976 901.632l309.0944-493.6192c3.7376-7.168 10.1376-7.1168 14.1312-0.2048l166.7584 287.9488c2.048 3.5328 6.5024 4.4544 9.7792 2.1504l141.4656-99.4816c6.6048-4.6592 15.4112-3.0208 19.8656 3.8912l257.0752 299.9808c4.352 6.8096 1.1776 12.3392-6.4512 12.3392L109.9264 914.6368C101.9904 914.6368 98.56 908.8512 102.2976 901.632z" p-id="2180" fill="#595959"></path><path d="M102.4 0l921.6 0 0 51.2-921.6 0 0-51.2Z" p-id="2181" fill="#595959"></path></svg>';

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

    const useDetail = (detail: any) => {
      editor.restoreSelection();

      const imageHtml = `<img src="${detail.logo}" alt="${detail.file_name}"/>`;

      editor.dangerouslyInsertHtml(imageHtml);
      imagesModal.destroy();
    };

    let res = await getAttachmentCategories();
    let categories = (res.data || []).reduce((pre: Object, cur: any) => {
      pre[cur.id] = cur.title;
      return pre;
    }, []);
    categories = Object.assign({ 0: '全部分类' }, categories);

    const imagesModal = Modal.confirm({
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
          className="material-table"
          rowKey="id"
          request={(params) => {
            console.log(params);
            return getAttachments(params);
          }}
          grid={{ gutter: 16, column: 6 }}
          pagination={{
            defaultPageSize: 24,
          }}
          showActions="hover"
          showExtra="hover"
          search={{
            span: 12,
            labelWidth: 120,
          }}
          rowClassName='image-row'
          metas={{
            content: {
              search: false,
              render: (text: any, row: any) => {
                return (
                  <div className="image-item">
                    <div className="inner">
                      {row.thumb ? (
                        <Image
                          className="img"
                          preview={{
                            src: row.logo,
                          }}
                          src={row.thumb}
                          alt={row.file_name}
                        />
                      ) : (
                        <Avatar className="default-img" size={100}>
                          {row.file_location.substring(row.file_location.lastIndexOf('.'))}
                        </Avatar>
                      )}
                      <div
                        className="info link"
                        onClick={() => {
                          useDetail(row);
                        }}
                      >
                        点击使用
                      </div>
                    </div>
                  </div>
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
export const ImagesMenuConf = {
  key: 'attachment', // menu key ，唯一。注册之后，可配置到工具栏
  factory() {
    return new ImageMenu();
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
