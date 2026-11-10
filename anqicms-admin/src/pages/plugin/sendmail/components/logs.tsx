import { pluginGetSendmails } from '@/services';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Modal } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef } from 'react';

export type SendMailLogsProps = {
  onCancel: (flag?: boolean) => void;
  open: boolean;
};

const PluginSendMailLogs: React.FC<SendMailLogsProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  useEffect(() => {}, []);

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.sendmail.send-time' }),
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'plugin.sendmail.recipient' }),
      dataIndex: 'address',
    },
    {
      title: intl.formatMessage({ id: 'plugin.sendmail.subject' }),
      dataIndex: 'subject',
    },
    {
      title: intl.formatMessage({ id: 'plugin.sendmail.status' }),
      width: 160,
      dataIndex: 'status',
    },
  ];

  return (
    <Modal
      width={1200}
      title={intl.formatMessage({ id: 'plugin.sendmail.logs' })}
      open={props.open}
      footer={null}
      onCancel={() => {
        props.onCancel();
      }}
    >
      <ProTable<any>
        rowKey="id"
        actionRef={actionRef}
        search={false}
        request={(params) => {
          return pluginGetSendmails(params);
        }}
        columnsState={{
          persistenceKey: 'sendmail-log-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
      />
    </Modal>
  );
};

export default PluginSendMailLogs;
