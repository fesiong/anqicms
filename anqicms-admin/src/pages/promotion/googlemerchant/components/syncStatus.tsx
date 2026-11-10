import {
  pluginGoogleMerchantSyncDelete,
  pluginGoogleMerchantSynclogs,
} from '@/services';
import { ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React from 'react';

export type GoogleMerchantSyncLogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

const GoogleMerchantSyncStatus: React.FC<GoogleMerchantSyncLogProps> = (
  props,
) => {
  const intl = useIntl();

  const handleRemove = (ids: number[]) => {
    pluginGoogleMerchantSyncDelete({
      ids: ids,
    }).then((res) => {
      message.success(res.msg);
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.product-id' }),
      dataIndex: 'product_id',
      render: (_, record) => {
        let text = record.product_id;
        if (record.sku_id > 0) {
          text += '(sku: ' + record.sku_id + ')';
        }
        return text;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.product-name' }),
      dataIndex: 'archive.title',
      render: (_, record) => {
        return record.archive?.title || '-';
      },
    },
    {
      title: intl.formatMessage({
        id: 'plugin.googlemerchant.google-product-id',
      }),
      dataIndex: 'google_product_id',
    },
    {
      title: intl.formatMessage({
        id: 'plugin.googlemerchant.google-status',
      }),
      dataIndex: 'google_status',
    },

    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.last-sync-time' }),
      width: 160,
      dataIndex: 'last_sync_time',
      render: (_, record) => {
        let text = dayjs(record.last_sync_time * 1000).format(
          'YYYY-MM-DD HH:mm',
        );
        if (record.last_message) {
          text += ': ' + record.last_message;
        }
        return text;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.merchant_id' }),
      dataIndex: 'merchant_id',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      width: 150,
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
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
    <Modal
      open={props.open}
      width={1000}
      onClose={() => props.onOpenChange(false)}
      onCancel={() => props.onOpenChange(false)}
      footer={false}
    >
      <ProTable<any>
        rowKey="id"
        search={false}
        toolBarRender={false}
        request={(params) => {
          return pluginGoogleMerchantSynclogs(params);
        }}
        columns={columns}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
    </Modal>
  );
};

export default GoogleMerchantSyncStatus;
