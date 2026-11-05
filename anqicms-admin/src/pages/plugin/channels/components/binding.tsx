import {
  channelBindingRevoke,
  channelBindingToken,
  channelBindings,
  channelList,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { useAccess, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Modal,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import CopyToClipboard from 'react-copy-to-clipboard';

const timeText = (seconds?: number) => {
  if (!seconds || seconds <= 0) {
    return '-';
  }
  return dayjs(seconds * 1000).format('YYYY-MM-DD HH:mm');
};

// 绑定 token 明文只存在于组件 state（R101）：
// 弹窗关闭、切换 tab（父级常驻挂载，靠 active 感知）或组件卸载时立即清空，
// 不写任何浏览器持久化存储，不打印日志，也不放进 URL。
const ChannelBinding: React.FC<{ active?: boolean }> = ({ active }) => {
  const intl = useIntl();
  const access = useAccess();
  const actionRef = useRef<ActionType>();
  const [channels, setChannels] = useState<any[]>([]);
  const [viewAll, setViewAll] = useState<boolean>(false);
  const [tokenVisible, setTokenVisible] = useState<boolean>(false);
  const [tokenData, setTokenData] = useState<any>(null);
  const [remaining, setRemaining] = useState<number>(0);
  // 记录是否已经激活过：首次激活由挂载负责拉取，重新激活才额外 reload，避免重复请求
  const activatedRef = useRef(false);

  const loadChannels = async () => {
    const res = await channelList();
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    setChannels(Array.isArray(res.data) ? res.data : []);
  };

  // 父级常驻挂载 + display 切换：
  // 离开本 tab 时立刻丢弃明文与弹窗状态；重新进入时重拉渠道与绑定列表
  useEffect(() => {
    if (active === false) {
      setTokenVisible(false);
      setTokenData(null);
      setRemaining(0);
      return;
    }
    loadChannels();
    if (activatedRef.current) {
      actionRef.current?.reload();
    }
    activatedRef.current = true;
  }, [active]);

  // 用服务端 expires_at 做倒计时，每秒刷新；到期后剩余秒数归零
  useEffect(() => {
    if (!tokenData || !tokenData.expires_at) {
      setRemaining(0);
      return;
    }
    const tick = () => {
      const left = Number(tokenData.expires_at) - Math.floor(Date.now() / 1000);
      setRemaining(left > 0 ? left : 0);
    };
    tick();
    const timer = setInterval(tick, 1000);
    return () => clearInterval(timer);
  }, [tokenData]);

  const closeToken = () => {
    setTokenVisible(false);
    setTokenData(null);
    setRemaining(0);
  };

  const onGenerateToken = async (values: any) => {
    const res = await channelBindingToken({ channel_id: values.channel_id });
    if (res.code !== 0) {
      message.error(res.msg);
      return false;
    }
    setTokenData(res.data || null);
    // 保持弹窗打开：明文只展示这一次，关闭后无法再次查看
    return false;
  };

  const remainText = () => {
    if (remaining <= 0) {
      return intl.formatMessage({
        id: 'plugin.channels.binding.token.expired',
      });
    }
    const minutes = Math.floor(remaining / 60);
    const seconds = remaining % 60;
    return `${minutes}:${String(seconds).padStart(2, '0')}`;
  };

  const channelName = (channelId: number) => {
    const channel = channels.find((item) => item.id === channelId);
    return channel?.name || (channelId ? String(channelId) : '-');
  };

  const handleRevoke = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.channels.binding.revoke.confirm' }),
      onOk: async () => {
        const res = await channelBindingRevoke({ id: record.id });
        if (res.code !== 0) {
          message.error(res.msg);
          return;
        }
        message.success(res.msg);
        actionRef.current?.reload();
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.binding.channel' }),
      dataIndex: 'channel_id',
      render: (_, record) => channelName(record.channel_id),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.binding.platform-user' }),
      dataIndex: 'platform_user_id',
      render: (_, record) => (
        <Typography.Text copyable={!!record.platform_user_id}>
          {record.platform_user_id || '-'}
        </Typography.Text>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.binding.admin' }),
      dataIndex: 'admin_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.binding.org' }),
      dataIndex: 'org_id',
      render: (_, record) => record.org_id || '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.binding.status' }),
      dataIndex: 'revoked_at',
      render: (_, record) =>
        record.revoked_at > 0 ? (
          <Space direction="vertical" size={0}>
            <Tag color="red">
              {intl.formatMessage({ id: 'plugin.channels.binding.revoked' })}
            </Tag>
            <span>{timeText(record.revoked_at)}</span>
          </Space>
        ) : (
          <Tag color="green">
            {intl.formatMessage({ id: 'plugin.channels.binding.active' })}
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.binding.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.action' }),
      dataIndex: 'action',
      render: (_, record) =>
        record.revoked_at > 0 ? (
          '-'
        ) : (
          <Button
            type="link"
            size="small"
            danger
            onClick={() => handleRevoke(record)}
          >
            {intl.formatMessage({ id: 'plugin.channels.binding.revoke' })}
          </Button>
        ),
    },
  ];

  return (
    <>
      <ProTable<any>
        rowKey="id"
        search={false}
        actionRef={actionRef}
        pagination={false}
        toolBarRender={() => [
          access.canSuper ? (
            <Space key="all">
              <span>
                {intl.formatMessage({ id: 'plugin.channels.binding.all' })}
              </span>
              <Switch
                checked={viewAll}
                onChange={(checked) => {
                  setViewAll(checked);
                }}
              />
            </Space>
          ) : null,
          <Button
            key="token"
            type="primary"
            onClick={() => {
              setTokenData(null);
              setRemaining(0);
              setTokenVisible(true);
            }}
          >
            {intl.formatMessage({ id: 'plugin.channels.binding.add' })}
          </Button>,
        ]}
        // 「查看全部」放进 params：pro-table 只在 params 变化时重发请求，
        // 只让开关写进 request 闭包会导致切开关没有任何反应
        params={{ all: viewAll && access.canSuper ? 1 : 0 }}
        request={async (params) => {
          // 默认只查本人；查看全部需要超级管理员，后端非超管返回 1002 时原样提示
          const res = await channelBindings({ all: params.all });
          if (res.code !== 0) {
            message.error(res.msg);
            return { data: [], success: false, total: 0 };
          }
          const list = Array.isArray(res.data) ? res.data : [];
          return { data: list, success: true, total: list.length };
        }}
        columns={columns}
      />
      {tokenVisible && (
        <ModalForm
          width={560}
          title={intl.formatMessage({ id: 'plugin.channels.binding.add' })}
          open={tokenVisible}
          layout="horizontal"
          labelCol={{ span: 6 }}
          onOpenChange={(flag) => {
            if (flag) {
              setTokenVisible(true);
            } else {
              closeToken();
            }
          }}
          onFinish={onGenerateToken}
        >
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message={intl.formatMessage({
              id: 'plugin.channels.binding.token.once',
            })}
          />
          <ProFormSelect
            name="channel_id"
            label={intl.formatMessage({ id: 'plugin.channels.binding.channel' })}
            options={channels.map((item) => ({
              label: item.name || String(item.id),
              value: item.id,
            }))}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.field.required',
                }),
              },
            ]}
          />
          {tokenData && (
            <Space
              direction="vertical"
              size={8}
              style={{ width: '100%', marginTop: 8 }}
            >
              <span>
                {intl.formatMessage({ id: 'plugin.channels.binding.token' })}
              </span>
              <Typography.Text code>{tokenData.token}</Typography.Text>
              <Space size={12} wrap>
                <CopyToClipboard
                  text={tokenData.token || ''}
                  onCopy={() => {
                    message.success(
                      intl.formatMessage({ id: 'plugin.channels.copied' }),
                    );
                  }}
                >
                  <Button size="small" disabled={remaining <= 0}>
                    {intl.formatMessage({ id: 'plugin.channels.copy' })}
                  </Button>
                </CopyToClipboard>
                <span>
                  {`${intl.formatMessage({
                    id: 'plugin.channels.binding.token.expires',
                  })}: ${remainText()}`}
                </span>
              </Space>
            </Space>
          )}
        </ModalForm>
      )}
    </>
  );
};

export default ChannelBinding;
