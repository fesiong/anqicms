import { channelDeliveryRetry, channelRecords } from '@/services';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Button, Modal, Segmented, Select, Space, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

const timeText = (seconds?: number) => {
  if (!seconds || seconds <= 0) {
    return '-';
  }
  return dayjs(seconds * 1000).format('YYYY-MM-DD HH:mm');
};

// 每种 type 的固定状态选项（取值与后端状态机常量一致；审计按 result 过滤）
const RECORD_TYPES = ['delivery', 'inbox', 'approval', 'audit'];

const STATUS_OPTIONS: Record<string, string[]> = {
  delivery: [
    'pending',
    'sending',
    'sent',
    'retry',
    'failed',
    'unknown',
    'cancelled',
  ],
  inbox: ['pending', 'processing', 'done', 'failed', 'cancelled'],
  approval: [
    'pending',
    'approved',
    'executing',
    'succeeded',
    'failed',
    'unknown',
    'rejected',
    'expired',
    'cancelled',
  ],
  audit: ['ok', 'failed', 'denied'],
};

// 运行记录（R106）：只展示后端白名单投影里的字段，不猜字段。
const ChannelRecords: React.FC = () => {
  const intl = useIntl();
  const actionRef = useRef<ActionType>();
  const [recordType, setRecordType] = useState<string>('delivery');
  const [status, setStatus] = useState<string>('');

  const statusLabel = (value: string) => {
    if (!value) {
      return '-';
    }
    const known = STATUS_OPTIONS.audit
      .concat(STATUS_OPTIONS.delivery, STATUS_OPTIONS.inbox, STATUS_OPTIONS.approval)
      .includes(value);
    if (!known) {
      return value;
    }
    return intl.formatMessage({
      id: `plugin.channels.records.status.${value}`,
    });
  };

  const handleRetry = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.channels.records.retry.confirm' }),
      onOk: async () => {
        const res = await channelDeliveryRetry({ id: record.id });
        if (res.code !== 0) {
          message.error(res.msg);
          return;
        }
        message.success(res.msg);
        actionRef.current?.reload();
      },
    });
  };

  const deliveryColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.status' }),
      dataIndex: 'status',
      render: (_, record) => <Tag>{statusLabel(record.status)}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.event' }),
      dataIndex: 'event_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.rule' }),
      dataIndex: 'rule_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.target' }),
      dataIndex: 'target_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.attempts' }),
      dataIndex: 'attempts',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.next-attempt' }),
      dataIndex: 'next_attempt_at',
      render: (_, record) => timeText(record.next_attempt_at),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.lease' }),
      dataIndex: 'lease_until',
      render: (_, record) => timeText(record.lease_until),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.last-error' }),
      dataIndex: 'last_error',
      ellipsis: true,
      render: (_, record) => record.last_error || '-',
    },
    {
      title: intl.formatMessage({
        id: 'plugin.channels.records.platform-message-id',
      }),
      dataIndex: 'platform_message_id',
      ellipsis: true,
      render: (_, record) => record.platform_message_id || '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.action' }),
      dataIndex: 'action',
      render: (_, record) =>
        // 只有失败或结果未知的投递才允许人工重试
        record.status === 'failed' || record.status === 'unknown' ? (
          <Button type="link" size="small" onClick={() => handleRetry(record)}>
            {intl.formatMessage({ id: 'plugin.channels.records.retry' })}
          </Button>
        ) : (
          '-'
        ),
    },
  ];

  const inboxColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.channel' }),
      dataIndex: 'channel_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.binding' }),
      dataIndex: 'binding_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.event-kind' }),
      dataIndex: 'event_kind',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.status' }),
      dataIndex: 'status',
      render: (_, record) => <Tag>{statusLabel(record.status)}</Tag>,
    },
    {
      // 入站任务被忽略/失败的原因（后端已脱敏，不含正文与平台标识原文）。
      title: intl.formatMessage({ id: 'plugin.channels.records.last_error' }),
      dataIndex: 'last_error',
      width: 200,
      ellipsis: true,
      render: (_, record) => record.last_error || '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.lease' }),
      dataIndex: 'lease_until',
      render: (_, record) => timeText(record.lease_until),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
  ];

  const approvalColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.binding' }),
      dataIndex: 'binding_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.session-id' }),
      dataIndex: 'session_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.tool' }),
      dataIndex: 'tool',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.action' }),
      dataIndex: 'action',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.status' }),
      dataIndex: 'status',
      render: (_, record) => <Tag>{statusLabel(record.status)}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.expires' }),
      dataIndex: 'expires_at',
      render: (_, record) => timeText(record.expires_at),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.policy-revision' }),
      dataIndex: 'policy_revision',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
  ];

  const auditColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.channel' }),
      dataIndex: 'channel_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.binding' }),
      dataIndex: 'binding_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.task' }),
      dataIndex: 'task_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.action' }),
      dataIndex: 'action',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.result' }),
      dataIndex: 'result',
      render: (_, record) => <Tag>{statusLabel(record.result)}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.records.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
  ];

  const columnsOf = (type: string) => {
    switch (type) {
      case 'inbox':
        return inboxColumns;
      case 'approval':
        return approvalColumns;
      case 'audit':
        return auditColumns;
      default:
        return deliveryColumns;
    }
  };

  return (
    <>
      <Space style={{ marginBottom: 16 }} wrap>
        <Segmented
          value={recordType}
          options={RECORD_TYPES.map((item) => ({
            label: intl.formatMessage({
              id: `plugin.channels.records.type.${item}`,
            }),
            value: item,
          }))}
          onChange={(value) => {
            setRecordType(String(value));
            setStatus('');
          }}
        />
        <Select
          style={{ width: 180 }}
          value={status}
          options={[
            {
              label: intl.formatMessage({
                id: 'plugin.channels.records.status.all',
              }),
              value: '',
            },
            ...(STATUS_OPTIONS[recordType] || []).map((item) => ({
              label: statusLabel(item),
              value: item,
            })),
          ]}
          onChange={(value: string) => {
            setStatus(value || '');
          }}
        />
      </Space>
      <ProTable<any>
        rowKey="id"
        search={false}
        actionRef={actionRef}
        params={{ type: recordType, status }}
        locale={{
          emptyText: intl.formatMessage({
            id: 'plugin.channels.records.empty',
          }),
        }}
        pagination={{
          defaultPageSize: 20,
          pageSizeOptions: [20, 50, 100],
          showSizeChanger: true,
        }}
        request={async (params) => {
          const res = await channelRecords({
            type: params.type || 'delivery',
            status: params.status || '',
            page: params.current || 1,
            // 上限 100 由后端兜底，这里只提供 20/50/100 三档
            page_size: params.pageSize || 20,
          });
          if (res.code !== 0) {
            message.error(res.msg);
            return { data: [], success: false, total: 0 };
          }
          const page = res.data || {};
          return {
            data: Array.isArray(page.list) ? page.list : [],
            success: true,
            total: page.total || 0,
          };
        }}
        columns={columnsOf(recordType)}
      />
    </>
  );
};

export default ChannelRecords;
