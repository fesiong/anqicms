import {
  getArchives,
  pluginAddCouponArchives,
  pluginDeleteCouponArchives,
  pluginGetCouponArchives,
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
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';

export type CouponArchivesProps = {
  coupon: any;
  open: boolean;
  onOpenChange: (flag: boolean) => void;
};

const CouponArchivesModal: React.FC<CouponArchivesProps> = (props) => {
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
          await pluginDeleteCouponArchives({
            archive_ids: selectedRowKeys,
            coupon_id: props.coupon.id,
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

  const handleAdd = async (values: any) => {
    const ids = values.ids || 0;
    if (ids.length > 0) {
      pluginAddCouponArchives({
        archive_ids: ids,
        coupon_id: props.coupon.id,
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
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
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
        title={
          props.coupon.range_type === 2
            ? intl.formatMessage({ id: 'plugin.coupon.not-product' })
            : intl.formatMessage({ id: 'plugin.coupon.product' })
        }
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
            params.coupon_id = props.coupon.id;
            const res = await pluginGetCouponArchives(params);
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
          title={intl.formatMessage({ id: 'plugin.coupon.archive' })}
          open={addOpen}
          onOpenChange={(flag) => setAddOpen(flag)}
          onFinish={handleAdd}
        >
          <ProFormSelect
            name="ids"
            showSearch
            mode="multiple"
            options={searchArchives.map((a: any) => ({
              title: a.title,
              label: a.title,
              value: a.id,
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

export default CouponArchivesModal;
