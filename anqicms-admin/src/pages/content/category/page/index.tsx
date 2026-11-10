import NewContainer from '@/components/NewContainer';
import { deleteCategory, getCategories } from '@/services/category';
import { PlusOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';
import '../index.less';

let lastParams: any = {};

const PageCategory: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const [isSubSite, setIsSubSite] = useState<boolean>(false);
  const intl = useIntl();

  const onTabChange = (key: string, isSubSite: boolean) => {
    setNewKey(key);
    setIsSubSite(isSubSite);
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.page.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteCategory({
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

  const handleEditCategory = async (record: any) => {
    history.push('/archive/page/detail?id=' + (record.id || 'new'));
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.sort.name' }),
      dataIndex: 'sort',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.page.name' }),
      dataIndex: 'title',
      hideInSearch: true,
      render: (dom, entity) => {
        return (
          <>
            {entity.spacer}
            <a href={entity.link} target="_blank" rel="noreferrer">
              {dom}
            </a>
          </>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.page.template.name' }),
      dataIndex: 'template',
      hideInSearch: true,
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
              handleEditCategory(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
              handleRemove([record.id]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={onTabChange}>
      <ProTable<any>
        key={newKey}
        headerTitle={intl.formatMessage({ id: 'menu.archive.page' })}
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          !isSubSite && (
            <Button
              type="primary"
              key="add"
              onClick={() => {
                handleEditCategory({ status: 1 });
              }}
            >
              <PlusOutlined /> <FormattedMessage id="content.page.new" />
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
          params.type = 3;
          lastParams = params;
          return getCategories(params);
        }}
        columnsState={{
          persistenceKey: 'page-table',
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
          showQuickJumper: true,
          defaultCurrent: lastParams.current,
          defaultPageSize: lastParams.pageSize,
        }}
      />
    </NewContainer>
  );
};

export default PageCategory;
