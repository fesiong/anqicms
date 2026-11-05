import {
  channelList,
  channelTargetSave,
  channelTargets,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Alert, Button, Empty, Select, Space, Switch, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

const timeText = (seconds?: number) => {
  if (!seconds || seconds <= 0) {
    return '-';
  }
  return dayjs(seconds * 1000).format('YYYY-MM-DD HH:mm');
};

// 会话目标（R104）：只允许创建群目标；私聊目标由绑定流程产生，页面只读展示。
// 不提供删除：取消批准用 approved=0（表格里直接切换）。
const ChannelTargets: React.FC = () => {
  const intl = useIntl();
  const actionRef = useRef<ActionType>();
  const [channels, setChannels] = useState<any[]>([]);
  const [channelId, setChannelId] = useState<number>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentTarget, setCurrentTarget] = useState<any>({});

  const loadChannels = async () => {
    const res = await channelList();
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    setChannels(Array.isArray(res.data) ? res.data : []);
  };

  useEffect(() => {
    loadChannels();
  }, []);

  const kindLabel = (kind: string) => {
    if (kind === 'group' || kind === 'private') {
      return intl.formatMessage({ id: `plugin.channels.target.kind.${kind}` });
    }
    return kind || '-';
  };

  // 开关与表单都提交完整字段：后端按整条记录更新，缺字段会被当成零值
  const submitTarget = async (record: any, changes: any) => {
    const res = await channelTargetSave({
      id: record.id || 0,
      channel_id: record.channel_id || channelId || 0,
      kind: record.kind || 'group',
      platform_target_id: record.platform_target_id || '',
      approved: record.approved || 0,
      management_enabled: record.management_enabled || 0,
      ...changes,
    });
    if (res.code !== 0) {
      message.error(res.msg);
      return false;
    }
    message.success(res.msg);
    actionRef.current?.reload();
    return true;
  };

  const openCreate = () => {
    setCurrentTarget({
      id: 0,
      channel_id: channelId || 0,
      kind: 'group',
      platform_target_id: '',
      approved: 0,
      management_enabled: 0,
    });
    setEditVisible(true);
  };

  const openEdit = (record: any) => {
    setCurrentTarget(record);
    setEditVisible(true);
  };

  const onSubmitTarget = async (values: any) => {
    const res = await channelTargetSave({
      id: currentTarget.id || 0,
      channel_id: currentTarget.channel_id || channelId || 0,
      // 私聊目标不允许手工创建或改类型，编辑只针对群目标
      kind: 'group',
      platform_target_id: values.platform_target_id,
      approved: values.approved,
      management_enabled: values.management_enabled,
    });
    if (res.code !== 0) {
      message.error(res.msg);
      return false;
    }
    message.success(res.msg);
    setEditVisible(false);
    actionRef.current?.reload();
    return true;
  };

  const flagCell = (record: any, dataIndex: 'approved' | 'management_enabled') => {
    // 私聊目标只读：改动必须由绑定流程或渠道管理完成
    if (record.kind !== 'group') {
      return record[dataIndex] === 1 ? (
        <Tag color="green">
          {intl.formatMessage({ id: 'plugin.channels.yes' })}
        </Tag>
      ) : (
        <Tag>{intl.formatMessage({ id: 'plugin.channels.no' })}</Tag>
      );
    }
    return (
      <Switch
        checked={record[dataIndex] === 1}
        onChange={(checked) =>
          submitTarget(record, { [dataIndex]: checked ? 1 : 0 })
        }
      />
    );
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.target.kind' }),
      dataIndex: 'kind',
      render: (_, record) => <Tag>{kindLabel(record.kind)}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.target.platform-id' }),
      dataIndex: 'platform_target_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.target.binding' }),
      dataIndex: 'binding_id',
      render: (_, record) => record.binding_id || '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.target.approved' }),
      dataIndex: 'approved',
      render: (_, record) => flagCell(record, 'approved'),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.target.management' }),
      dataIndex: 'management_enabled',
      render: (_, record) => flagCell(record, 'management_enabled'),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.action' }),
      dataIndex: 'action',
      render: (_, record) =>
        record.kind === 'group' ? (
          <Button type="link" size="small" onClick={() => openEdit(record)}>
            {intl.formatMessage({ id: 'plugin.channels.action.edit' })}
          </Button>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <>
      <Space style={{ marginBottom: 16 }} wrap>
        <span>{intl.formatMessage({ id: 'plugin.channels.target.channel' })}</span>
        <Select
          style={{ width: 240 }}
          value={channelId}
          placeholder={intl.formatMessage({
            id: 'plugin.channels.target.select-channel',
          })}
          options={channels.map((item) => ({
            label: item.name || String(item.id),
            value: item.id,
          }))}
          onChange={(value: number) => {
            setChannelId(value);
            setEditVisible(false);
          }}
        />
        <Button type="primary" disabled={!channelId} onClick={() => openCreate()}>
          {intl.formatMessage({ id: 'plugin.channels.target.add' })}
        </Button>
      </Space>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message={intl.formatMessage({
          id: 'plugin.channels.target.private-hint',
        })}
      />
      {channelId ? (
        <ProTable<any>
          rowKey="id"
          search={false}
          actionRef={actionRef}
          pagination={false}
          locale={{
            emptyText: intl.formatMessage({
              id: 'plugin.channels.target.empty',
            }),
          }}
          request={async () => {
            const res = await channelTargets({ channel_id: channelId });
            if (res.code !== 0) {
              message.error(res.msg);
              return { data: [], success: false, total: 0 };
            }
            const list = Array.isArray(res.data) ? res.data : [];
            return { data: list, success: true, total: list.length };
          }}
          columns={columns}
        />
      ) : (
        <Empty
          description={intl.formatMessage({
            id: 'plugin.channels.target.select-channel',
          })}
        />
      )}
      {editVisible && (
        <ModalForm
          width={560}
          title={
            currentTarget.id
              ? intl.formatMessage({ id: 'plugin.channels.target.edit' })
              : intl.formatMessage({ id: 'plugin.channels.target.add' })
          }
          open={editVisible}
          layout="horizontal"
          labelCol={{ span: 6 }}
          initialValues={{
            kind: 'group',
            platform_target_id: currentTarget.platform_target_id || '',
            approved: currentTarget.approved || 0,
            management_enabled: currentTarget.management_enabled || 0,
          }}
          onOpenChange={(flag) => {
            setEditVisible(flag);
          }}
          onFinish={onSubmitTarget}
        >
          <ProFormText
            name="kind"
            label={intl.formatMessage({ id: 'plugin.channels.target.kind' })}
            disabled
            extra={intl.formatMessage({
              id: 'plugin.channels.target.kind.group',
            })}
          />
          <ProFormText
            name="platform_target_id"
            label={intl.formatMessage({
              id: 'plugin.channels.target.platform-id',
            })}
            extra={intl.formatMessage({
              id: 'plugin.channels.target.platform-id.description',
            })}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.field.required',
                }),
              },
            ]}
          />
          <ProFormRadio.Group
            name="approved"
            label={intl.formatMessage({ id: 'plugin.channels.target.approved' })}
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.channels.yes' }),
                value: 1,
              },
              {
                label: intl.formatMessage({ id: 'plugin.channels.no' }),
                value: 0,
              },
            ]}
          />
          <ProFormRadio.Group
            name="management_enabled"
            label={intl.formatMessage({
              id: 'plugin.channels.target.management',
            })}
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.channels.yes' }),
                value: 1,
              },
              {
                label: intl.formatMessage({ id: 'plugin.channels.no' }),
                value: 0,
              },
            ]}
          />
        </ModalForm>
      )}
    </>
  );
};

export default ChannelTargets;
