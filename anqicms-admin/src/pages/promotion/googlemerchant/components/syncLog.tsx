import { pluginGoogleMerchantSynclogs } from '@/services';
import { ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Modal } from 'antd';
import dayjs from 'dayjs';
import React from 'react';

export type GoogleMerchantSyncLogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

const GoogleMerchantSyncLog: React.FC<GoogleMerchantSyncLogProps> = (props) => {
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.log.time' }),
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.merchant_id' }),
      dataIndex: 'merchant_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.sync-type' }),
      dataIndex: 'sync_type',
    },
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.id-range' }),
      dataIndex: 'start_id',
      render: (text, record) => {
        return record.start_id + '-' + record.end_id;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.googlemerchant.sync-status' }),
      dataIndex: 'status',
      render: (_, record) => {
        let text = '';
        if (record.status === 2) {
          text = intl.formatMessage({
            id: 'plugin.googlemerchant.sync-status.success',
          });
        } else if (record.status === 3) {
          text = intl.formatMessage({
            id: 'plugin.googlemerchant.sync-status.failed',
          });
        } else if (record.status === 1) {
          text = intl.formatMessage({
            id: 'plugin.googlemerchant.sync-status.processing',
          });
        }
        if (record.message) {
          text += ': ' + record.message;
        }
        return text;
      },
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

export default GoogleMerchantSyncLog;
