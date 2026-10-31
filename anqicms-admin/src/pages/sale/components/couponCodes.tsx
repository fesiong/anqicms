import {
  pluginDeleteCouponCode,
  pluginGetCouponCodes,
  pluginGetUsers,
  pluginMakeCouponCode,
  pluginSaveCouponCode,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

export type CouponCodesProps = {
  coupon: any;
  open: boolean;
  onOpenChange: (flag: boolean) => void;
};

const CouponCodesModal: React.FC<CouponCodesProps> = (props) => {
  const intl = useIntl();
  const actionRef = useRef<ActionType>();
  const [addOpen, setAddOpen] = useState<boolean>(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [bindOpen, setBindOpen] = useState<boolean>(false);
  const [currentCode, setCurrentCode] = useState<any>({});
  const [searchUsers, setSearchUsers] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'content.parent_id.empty',
      }),
    },
  ]);

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
          await pluginDeleteCouponCode({
            ids: selectedRowKeys,
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

  const handleShowBind = (record: any) => {
    setCurrentCode(record);
    setBindOpen(true);
  };

  const handleBind = async (values: any) => {
    pluginSaveCouponCode({
      id: currentCode.id,
      coupon_id: props.coupon.id,
      user_id: values.user_id,
      code: currentCode.code,
    }).then(() => {
      actionRef.current?.reload();
      setBindOpen(false);
    });
  };

  const handleAdd = async (values: any) => {
    pluginMakeCouponCode({
      user_id: values.user_id,
      count: values.count,
      coupon_id: props.coupon.id,
    }).then(() => {
      actionRef.current?.reload();
      setAddOpen(false);
    });
  };

  const onSearchUsers = (e: any) => {
    pluginGetUsers({ q: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [];
      searchItems.push({
        id: 0,
        title: intl.formatMessage({
          id: 'content.parent_id.empty',
        }),
      });
      setSearchUsers(searchItems.concat(res.data || []));
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.code' }),
      dataIndex: 'code',
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.amount' }),
      dataIndex: 'money',
      render: (_: any, record) => {
        return (
          <div>
            {record.money_type === 2
              ? record.money +
                '%' +
                (record.max_money > 0
                  ? ', ' +
                    intl.formatMessage({ id: 'plugin.coupon.amount.max' }) +
                    (record.max_money / 100).toFixed(1)
                  : '')
              : record.money_type === 1
              ? (record.money / 100).toFixed(1) +
                ' - ' +
                (record.max_money / 100).toFixed(1)
              : (record.money / 100).toFixed(1)}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.user-name' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.get-time' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'get_time',
      render: (_, record) => {
        return (
          <div>
            {record.get_time > 0
              ? dayjs(record.get_time * 1000).format('YYYY-MM-DD HH:mm')
              : '-'}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.use-time' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'use_time',
      render: (_, record) => {
        return (
          <div>
            {record.use_time > 0
              ? dayjs(record.use_time * 1000).format('YYYY-MM-DD HH:mm')
              : '-'}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.end-time' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'end_time',
      render: (_, record) => {
        return <div>{dayjs(record.end_time * 1000).format('YYYY-MM-DD')}</div>;
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          {record.user_id === 0 && (
            <a
              onClick={async () => {
                await handleShowBind(record);
              }}
            >
              <FormattedMessage id="plugin.coupon.bind-user" />
            </a>
          )}
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
        width={1200}
        title={intl.formatMessage({ id: 'plugin.coupon.code' })}
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
              <FormattedMessage id="plugin.coupon.code.generate" />
            </Button>,
          ]}
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
            const res = await pluginGetCouponCodes(params);
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
          title={intl.formatMessage({ id: 'plugin.coupon.code.generate' })}
          open={addOpen}
          onOpenChange={(flag) => setAddOpen(flag)}
          onFinish={handleAdd}
          width={480}
        >
          <ProFormSelect
            name="user_id"
            label={intl.formatMessage({ id: 'plugin.coupon.bind-user' })}
            showSearch
            options={searchUsers.map((a: any) => ({
              title: a.user_name,
              label: a.user_name,
              value: a.id,
            }))}
            fieldProps={{
              onSearch: (e) => {
                onSearchUsers(e);
              },
            }}
            extra={intl.formatMessage({
              id: 'plugin.coupon.code.generate.description',
            })}
          />
          <ProFormDigit
            name="count"
            label={intl.formatMessage({ id: 'plugin.coupon.code.count' })}
            min={1}
            extra={intl.formatMessage({
              id: 'plugin.coupon.code.count.description',
            })}
          />
        </ModalForm>
      )}
      {bindOpen && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.coupon.bind-user' })}
          open={bindOpen}
          onOpenChange={(flag) => setBindOpen(flag)}
          onFinish={handleBind}
          width={480}
        >
          <ProFormSelect
            name="user_id"
            showSearch
            options={searchUsers.map((a: any) => ({
              title: a.user_name,
              label: a.user_name,
              value: a.id,
            }))}
            fieldProps={{
              onSearch: (e) => {
                onSearchUsers(e);
              },
            }}
          />
        </ModalForm>
      )}
    </>
  );
};

export default CouponCodesModal;
