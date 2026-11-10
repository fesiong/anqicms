import {
  getArchives,
  updateArchiveParent,
  updateArchivesSort,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Input, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';
import './index.less';

export type ChildrenArchivesProps = {
  parent: any;
  open: boolean;
  onOpenChange: (flag: boolean) => void;
};

let updating = false;

const ChildrenArchivesModal: React.FC<ChildrenArchivesProps> = (props) => {
  const intl = useIntl();
  const actionRef = useRef<ActionType>();
  const [addOpen, setAddOpen] = useState<boolean>(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [searchArchives, setSearchArchives] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'content.parent_id.empty',
      }),
    },
  ]);

  const onSearchArchives = (e: any) => {
    getArchives({ title: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [
        {
          id: 0,
          title: intl.formatMessage({
            id: 'content.parent_id.empty',
          }),
        },
      ];
      setSearchArchives(searchItems.concat(res.data || []));
    });
  };

  const updateSort = (index: number, record: any, value: any) => {
    if (updating) {
      return;
    }
    let sortNum = parseInt(value);
    if (isNaN(sortNum)) {
      message.error(intl.formatMessage({ id: 'content.sort.required' }));
      return;
    }
    if (sortNum === record.sort) {
      return;
    }
    if (sortNum < 0) {
      message.error(intl.formatMessage({ id: 'content.sort.required' }));
      return;
    }
    updating = true;
    updateArchivesSort({
      sort: sortNum,
      id: record.id,
    })
      .then((res) => {
        message.success(res.msg);
        actionRef.current?.reload?.();
      })
      .finally(() => {
        updating = false;
      });
  };

  const handleRemove = (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          {
            content: intl.formatMessage({ id: 'content.delete.deletting' }),
            key: 'delete',
          },
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          await updateArchiveParent({
            ids: selectedRowKeys,
            parent_id: 0,
          });
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

  const handleAddChildren = async (values: any) => {
    const ids = values.ids || 0;
    if (ids.length > 0) {
      updateArchiveParent({
        ids: ids,
        parent_id: props.parent.id,
      }).then((res) => {
        actionRef.current?.reload();
      });
    }
    setAddOpen(false);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: intl.formatMessage({ id: 'content.sort.name' }),
      dataIndex: 'sort',
      hideInSearch: true,
      sorter: true,
      tooltip: intl.formatMessage({ id: 'content.sort.tips' }),
      width: 90,
      render: (_, entity: any, index) => {
        return (
          <div>
            <Input
              onBlur={(e: any) => {
                updateSort(index, entity, e.target.value);
              }}
              onPressEnter={(e: any) => {
                updateSort(index, entity, e.target?.value);
              }}
              defaultValue={entity.sort}
            ></Input>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
      render: (dom, entity) => {
        return (
          <div style={{ maxWidth: 400 }}>
            <a href={entity.link} target="_blank" rel="noreferrer">
              {dom}
            </a>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.module.name' }),
      dataIndex: 'module_id',
      render: (_: any, entity) => {
        return entity.module_name;
      },
    },
    {
      title: intl.formatMessage({ id: 'content.category.name' }),
      dataIndex: 'category_titles',
      render: (_: any, entity) => {
        return (
          <div>
            {entity.category_titles?.map((item: string) => (
              <div key={item}>{item}</div>
            ))}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.status.draft' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'content.status.normal' }),
          status: 'Success',
        },
        2: {
          text: intl.formatMessage({ id: 'content.status.plan' }),
          status: 'Warning',
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
            className="text-red"
            onClick={async () => {
              await handleRemove([record.id]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Modal
        open={props.open}
        width={1100}
        title={intl.formatMessage({ id: 'content.children.btn' })}
        onCancel={() => props.onOpenChange(false)}
        onOk={() => props.onOpenChange(false)}
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          search={false}
          toolBarRender={() => [
            <Button
              type="primary"
              key="add"
              onClick={() => {
                setAddOpen(true);
              }}
            >
              <PlusOutlined /> <FormattedMessage id="content.archive.add" />
            </Button>,
          ]}
          columnsState={{
            persistenceKey: 'archive-table',
            persistenceType: 'localStorage',
          }}
          tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
            <Space>
              <Button
                size={'small'}
                onClick={async () => {
                  await handleRemove(selectedRowKeys);
                }}
              >
                <FormattedMessage id="content.option.batch-delete" />
              </Button>
              <Button type="link" size={'small'} onClick={onCleanSelected}>
                <FormattedMessage id="content.option.cancel-select" />
              </Button>
            </Space>
          )}
          request={async (params) => {
            params.exact = true;
            params.sort = 'sort desc,created_time';
            params.order = 'desc';
            params.parent_id = props.parent.id;
            const res = await getArchives(params);
            return res;
          }}
          columns={columns}
          rowSelection={{
            preserveSelectedRowKeys: true,
            onChange: (selectedRowKeys) => {
              setSelectedRowKeys(selectedRowKeys);
            },
          }}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Modal>
      {addOpen && (
        <ModalForm
          title={intl.formatMessage({ id: 'content.children.btn' })}
          open={addOpen}
          onOpenChange={(flag) => setAddOpen(flag)}
          onFinish={handleAddChildren}
        >
          <ProFormSelect
            name="ids"
            showSearch
            mode="multiple"
            options={searchArchives.map((a: any) => ({
              title: a.title,
              label: a.title,
              value: a.id,
              disabled: a.id === props.parent.id,
            }))}
            fieldProps={{
              onSearch: (e) => {
                onSearchArchives(e);
              },
            }}
          />
        </ModalForm>
      )}
    </>
  );
};

export default ChildrenArchivesModal;
