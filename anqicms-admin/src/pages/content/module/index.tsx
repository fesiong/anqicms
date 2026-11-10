import NewContainer from '@/components/NewContainer';
import { deleteModule, getModules } from '@/services';
import { getSessionStore } from '@/utils/store';
import { PlusOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Modal, Space, Tag, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import ModuleForm from './components/moduleForm';

const ModuleList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentModule, setCurrentModule] = useState<any>({});
  const [newKey, setNewKey] = useState<string>('');
  const [isSubSite, setIsSubSite] = useState<boolean>(false);
  const intl = useIntl();

  const onTabChange = (key: string, isSubSite: boolean) => {
    setNewKey(key);
    setIsSubSite(isSubSite);
  };

  useEffect(() => {
    setIsSubSite(getSessionStore('is-sub-site'));
  }, []);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.module.delete.confirm' }),
      content: intl.formatMessage({ id: 'content.module.delete.content' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteModule({
              id: item,
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleEditModule = async (record: any) => {
    setCurrentModule(record);
    setEditVisible(true);
  };

  const handleShowArchive = async (record: any) => {
    history.push('/archive/list?module_id=' + record.id);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.module.title' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'content.module.title-name' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'content.module.field' }),
      dataIndex: 'table_name',
    },
    {
      // 类型决定前台按文章还是产品渲染（价格/库存/SKU/Product 结构化数据），
      // 单看表名认不出来，列表里直接标出来。
      title: intl.formatMessage({ id: 'content.module.kind' }),
      dataIndex: 'kind',
      hideInSearch: true,
      render: (_, record) => (
        <Tag color={record.kind === 'product' ? 'gold' : 'blue'}>
          {intl.formatMessage({
            id:
              record.kind === 'product'
                ? 'content.module.kind.product'
                : 'content.module.kind.article',
          })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'content.module.issystem' }),
      dataIndex: 'is_system',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.module.issystem.no' }),
        },
        1: {
          text: intl.formatMessage({ id: 'content.module.issystem.yes' }),
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'content.category.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.category.status.hide' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'content.category.status.ok' }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleShowArchive(record);
            }}
          >
            <FormattedMessage id="menu.archive.list" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditModule(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          {record.is_system === 0 && (
            <a
              className="text-red"
              key="delete"
              onClick={() => {
                handleRemove([record.id]);
              }}
            >
              <FormattedMessage id="setting.system.delete" />
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={onTabChange}>
      <ProTable<any>
        key={newKey}
        headerTitle={intl.formatMessage({ id: 'menu.archive.module' })}
        actionRef={actionRef}
        rowKey="id"
        search={{}}
        toolBarRender={() => [
          !isSubSite && (
            <Button
              type="primary"
              key="add"
              onClick={() => {
                handleEditModule({ status: 1 });
              }}
            >
              <PlusOutlined /> <FormattedMessage id="content.module.add" />
            </Button>
          ),
        ]}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
            <Button
              size={'small'}
              onClick={() => {
                handleRemove(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-delete" />
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              <FormattedMessage id="content.option.cancel-select" />
            </Button>
          </Space>
        )}
        request={(params) => {
          return getModules(params);
        }}
        columnsState={{
          persistenceKey: 'module-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          showSizeChanger: true,
        }}
      />
      {editVisible && (
        <ModuleForm
          open={editVisible}
          module={currentModule}
          type={1}
          onCancel={() => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
          onSubmit={async () => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
    </NewContainer>
  );
};

export default ModuleList;
