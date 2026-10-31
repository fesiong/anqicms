import NewContainer from '@/components/NewContainer';
import { getCategories } from '@/services';
import { deleteTag, getTags } from '@/services/tag';
import { PlusOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';
import BatchForm from './components/batchForm';
import TagFieldsForm from './components/fields';
import './index.less';

const ArticleTag: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [batchVisible, setBatchVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const [isSubSite, setIsSubSite] = useState<boolean>(false);
  const [fieldsVisible, setFieldsVisible] = useState<boolean>(false);
  const intl = useIntl();

  const onTabChange = (key: string, isSubSite: boolean) => {
    setNewKey(key);
    setIsSubSite(isSubSite);
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.tags.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteTag({
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

  const handleEditTag = async (record: any) => {
    history.push('/archive/tag/detail?id=' + (record.id || 'new'));
  };

  const handleAddTags = () => {
    setBatchVisible(true);
  };

  const handleEditFields = () => {
    setFieldsVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.tags.name' }),
      dataIndex: 'title',
      render: (dom, entity) => {
        return (
          <a href={entity.link} target="_blank" rel="noreferrer">
            {dom}
          </a>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.category.name' }),
      dataIndex: 'category_title',
      request: async () => {
        let res = await getCategories({ type: 1 });
        const categories = [
          {
            parents: [],
            title: intl.formatMessage({ id: 'content.category.all' }),
            id: 0,
            status: 1,
          },
        ]
          .concat(res.data || [])
          .map((cat: any) => ({
            title: cat.title,
            label: (
              <div title={cat.title}>
                {cat.parents?.length > 0 ? (
                  <span className="text-muted">
                    {cat.parents
                      ?.map((parent: any) => parent.title)
                      .join(' > ')}
                    {' > '}
                  </span>
                ) : (
                  ''
                )}
                {cat.title}
              </div>
            ),
            value: cat.id,
            disabled: cat.status !== 1,
          }));
        return categories;
      },
      fieldProps: {
        showSearch: true,
        filterOption: (input: string, option: any) =>
          (option?.title ?? option?.label)
            .toLowerCase()
            .includes(input.toLowerCase()),
      },
    },
    {
      title: intl.formatMessage({ id: 'content.tags.first-letter.name' }),
      dataIndex: 'first_letter',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.description.name' }),
      dataIndex: 'description',
      hideInSearch: true,
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
              handleEditTag(record);
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
        headerTitle={intl.formatMessage({ id: 'menu.archive.tag' })}
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button
            type="default"
            key="fields"
            onClick={() => {
              handleEditFields();
            }}
          >
            <FormattedMessage id="content.tag.fields" />
          </Button>,
          !isSubSite && (
            <Button
              type="default"
              key="add2"
              onClick={() => {
                handleAddTags();
              }}
            >
              <FormattedMessage id="content.tags.batch-add" />
            </Button>
          ),
          !isSubSite && (
            <Button
              type="primary"
              key="add"
              onClick={() => {
                handleEditTag({});
              }}
            >
              <PlusOutlined /> <FormattedMessage id="content.tags.add" />
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
          params.type = 1;
          return getTags(params);
        }}
        columnsState={{
          persistenceKey: 'archive-tag-table',
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
        }}
      />
      {batchVisible && (
        <BatchForm
          open={batchVisible}
          onCancel={() => {
            setBatchVisible(false);
          }}
          onSubmit={async () => {
            setBatchVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
      {fieldsVisible && (
        <TagFieldsForm
          open={fieldsVisible}
          onCancel={() => {
            setFieldsVisible(false);
          }}
          onSubmit={async () => {
            setFieldsVisible(false);
          }}
        />
      )}
    </NewContainer>
  );
};

export default ArticleTag;
