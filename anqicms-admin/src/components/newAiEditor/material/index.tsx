import {
  pluginGetMaterialCategories,
  pluginGetMaterials,
} from '@/services/plugin/material';
import {
  EditOutlined,
  FileSearchOutlined,
  PlusSquareOutlined,
} from '@ant-design/icons';
import { ProList } from '@ant-design/pro-components';
import { Link, useIntl } from '@umijs/max';
import { Modal, Space, Tag } from 'antd';
import { useEffect, useState } from 'react';
import './index.less';

export type MaterialProps = {
  onSelect: (row: MaterialElement) => void;
  onCancel?: (row?: any) => void;
  open?: boolean;
};

export type MaterialElement = {
  type: 'material';
  id: string;
  title: string;
  content: string;
};

// 定义菜单 class
const MaterialSelect: React.FC<MaterialProps> = (props) => {
  const [categories, setCategories] = useState<any>([]);
  const intl = useIntl();

  useEffect(() => {
    pluginGetMaterialCategories().then((res) => {
      let categories = (res.data || []).reduce((pre: any, cur: any) => {
        pre[cur.id] = cur.title;
        return pre;
      }, []);
      setCategories(categories);
    });
  }, []);
  const handlePreview = (detail: any) => {
    Modal.info({
      title: detail.title,
      icon: null,
      width: 600,
      content: <div dangerouslySetInnerHTML={{ __html: detail.content }}></div>,
    });
  };

  return (
    <>
      <Modal
        title={
          <Space size={30}>
            <span>
              {intl.formatMessage({ id: 'component.editor.insert-material' })}
            </span>
            <Link className="link" to={'/plugin/contentops/material'}>
              <EditOutlined />
            </Link>
          </Space>
        }
        width={800}
        open={props.open}
        footer={null}
        onCancel={() => {
          props.onCancel?.();
        }}
      >
        <ProList<any>
          className="material-table"
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
                        handlePreview(row);
                      }}
                    >
                      <FileSearchOutlined />{' '}
                      {intl.formatMessage({ id: 'component.material.preview' })}
                    </Tag>
                    <Tag
                      color="blue"
                      className="link"
                      onClick={() => {
                        props.onSelect(row);
                      }}
                    >
                      <PlusSquareOutlined />{' '}
                      {intl.formatMessage({ id: 'component.material.use' })}
                    </Tag>
                  </Space>
                );
              },
            },
            category_id: {
              title: intl.formatMessage({ id: 'component.material.category' }),
              valueType: 'select',
              valueEnum: categories,
            },
          }}
        />
      </Modal>
    </>
  );
};

export default MaterialSelect;
