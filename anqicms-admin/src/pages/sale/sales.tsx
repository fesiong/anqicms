import { pluginDeleteCoupon, pluginGetCoupons } from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import CouponArchivesModal from './components/archives';
import CouponCodesModal from './components/couponCodes';
import CouponForm from './components/form';

// 只显示满减
const CouponType = 4;
const PluginCouponSales: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentCoupon, setCurrentCoupon] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [archivesVisible, setArchivesVisible] = useState<boolean>(false);
  const [codesVisible, setCodesVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.keyword.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let i = 0; i < selectedRowKeys.length; i++) {
            await pluginDeleteCoupon({
              id: selectedRowKeys[i],
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          if (actionRef.current) {
            actionRef.current.reloadAndRest?.();
          }
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleEditCoupon = async (record: any) => {
    setCurrentCoupon(record);
    setEditVisible(true);
  };

  const handleShowArchives = (record: any) => {
    setCurrentCoupon(record);
    setArchivesVisible(true);
  };

  const handleShowCodes = (record: any) => {
    setCurrentCoupon(record);
    setCodesVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      hideInSearch: true,
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.description' }),
      hideInSearch: true,
      dataIndex: 'description',
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.type' }),
      dataIndex: 'type',
      valueEnum: {
        1: {
          text: intl.formatMessage({ id: 'plugin.coupon.type.register' }),
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.coupon.type.limited' }),
        },
        3: {
          text: intl.formatMessage({ id: 'plugin.coupon.type.order' }),
        },
        4: {
          text: intl.formatMessage({ id: 'plugin.coupon.type.discount' }),
        },
        5: {
          text: intl.formatMessage({ id: 'plugin.coupon.type.unlimited' }),
        },
        6: {
          text: intl.formatMessage({
            id: 'plugin.coupon.type.integral',
          }),
        },
        7: {
          text: intl.formatMessage({ id: 'plugin.coupon.type.activity' }),
        },
      },
      renderFormItem: () => {
        return (
          <ProFormSelect
            name="type"
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.register',
                }),
                value: 1,
              },
              {
                label: intl.formatMessage({ id: 'plugin.coupon.type.limited' }),
                value: 2,
              },
              {
                label: intl.formatMessage({ id: 'plugin.coupon.type.order' }),
                value: 3,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.discount',
                }),
                value: 4,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.unlimited',
                }),
                value: 5,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.integral',
                }),
                value: 6,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.activity',
                }),
                value: 7,
              },
            ]}
          />
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.amount' }),
      hideInSearch: true,
      dataIndex: 'money',
      render: (_, record) => (
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
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.start-time' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'start_time',
      render: (_, record) => {
        return (
          <div>
            {dayjs(record.start_time * 1000).format('YYYY-MM-DD')} -
            {dayjs(record.end_time * 1000).format('YYYY-MM-DD')}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.expire' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'expire',
      render: (dom) => {
        return (
          <div>
            {dom} <FormattedMessage id="plugin.coupon.expire.suffix" />
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.coupon.status.hide' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.coupon.status.show' }),
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
          {record.range_type === 2 ? (
            <a
              key="collect"
              onClick={() => {
                handleShowArchives(record);
              }}
            >
              <FormattedMessage id="plugin.coupon.not-product" />
            </a>
          ) : record.range_type === 1 ? (
            <a
              key="collect"
              onClick={() => {
                handleShowArchives(record);
              }}
            >
              <FormattedMessage id="plugin.coupon.product" />{' '}
            </a>
          ) : null}

          {record.type !== 4 && (
            <a
              key="collect"
              onClick={() => {
                handleShowCodes(record);
              }}
            >
              <FormattedMessage id="plugin.coupon.code" />
            </a>
          )}
          <a
            key="edit"
            onClick={() => {
              handleEditCoupon(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
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
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'menu.sale.coupon' })}
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditCoupon({ status: 1, type: CouponType });
            }}
          >
            <PlusOutlined /> <FormattedMessage id="plugin.coupon.add" />
          </Button>,
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
          params.type = CouponType;
          return pluginGetCoupons(params);
        }}
        columnsState={{
          persistenceKey: 'plugin-coupons-table',
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
      {editVisible && (
        <CouponForm
          open={editVisible}
          coupon={currentCoupon}
          onCancel={() => {
            setEditVisible(false);
          }}
          onSubmit={async () => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
      {archivesVisible && (
        <CouponArchivesModal
          coupon={currentCoupon}
          open={archivesVisible}
          onOpenChange={(flag) => {
            setArchivesVisible(flag);
          }}
        />
      )}
      {codesVisible && (
        <CouponCodesModal
          coupon={currentCoupon}
          open={codesVisible}
          onOpenChange={(flag) => {
            setCodesVisible(flag);
          }}
        />
      )}
    </PageContainer>
  );
};

export default PluginCouponSales;
