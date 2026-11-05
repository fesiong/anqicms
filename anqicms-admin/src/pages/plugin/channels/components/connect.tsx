import {
  channelCapabilities,
  channelDelete,
  channelDetail,
  channelList,
  channelSave,
  channelStatus,
  channelTargets,
  channelTest,
  getAiAgents,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormDependency,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Descriptions,
  Drawer,
  Modal,
  Select,
  Space,
  Spin,
  Tag,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

const timeText = (seconds?: number) => {
  if (!seconds || seconds <= 0) {
    return '-';
  }
  return dayjs(seconds * 1000).format('YYYY-MM-DD HH:mm');
};

const ChannelConnect: React.FC = () => {
  const intl = useIntl();
  const actionRef = useRef<ActionType>();
  const formRef = React.createRef<ProFormInstance>();
  const [agents, setAgents] = useState<any[]>([]);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentChannel, setCurrentChannel] = useState<any>({});
  const [statusVisible, setStatusVisible] = useState<boolean>(false);
  const [statusLoading, setStatusLoading] = useState<boolean>(false);
  const [statusChannel, setStatusChannel] = useState<any>({});
  const [statusData, setStatusData] = useState<any>(null);
  const [testVisible, setTestVisible] = useState<boolean>(false);
  const [testChannel, setTestChannel] = useState<any>({});
  const [testOptions, setTestOptions] = useState<any[]>([]);
  const [capVisible, setCapVisible] = useState<boolean>(false);
  const [capLoading, setCapLoading] = useState<boolean>(false);
  const [capAgentId, setCapAgentId] = useState<number>();
  const [capData, setCapData] = useState<any>({
    intents: [],
    blocked_builtins: [],
  });

  // 智能体列表是既有接口（/anqi/ai/agents），接待智能体与分析智能体共用
  const loadAgents = async () => {
    try {
      const res = await getAiAgents();
      if (Array.isArray(res.data)) {
        setAgents(res.data);
      }
    } catch (e) {
      // 智能体列表拉取失败不阻断页面，表单里会提示暂无可选智能体
      setAgents([]);
    }
  };

  useEffect(() => {
    loadAgents();
  }, []);

  const agentName = (agentId: number) => {
    if (!agentId) {
      return '-';
    }
    const agent = agents.find((item) => item.id === agentId);
    return agent?.name || String(agentId);
  };

  const agentOptions = agents.map((item) => ({
    label: item.name || String(item.id),
    value: item.id,
  }));

  // 平台只有飞书/钉钉两种，未知值原样显示，避免多语言缺失报警
  const platformLabel = (platform: string) => {
    if (platform === 'feishu' || platform === 'dingtalk') {
      return intl.formatMessage({ id: `plugin.channels.platform.${platform}` });
    }
    return platform || '-';
  };

  const openCreate = () => {
    setCurrentChannel({
      id: 0,
      platform: 'feishu',
      enabled: 0,
      chat_enabled: 0,
      agent_id: 0,
      revision: 0,
      policy_json: '',
      app_secret: '',
    });
    setEditVisible(true);
  };

  const openEdit = async (record: any) => {
    const res = await channelDetail({ id: record.id });
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    setCurrentChannel(res.data || {});
    setEditVisible(true);
  };

  const onSubmitChannel = async (values: any) => {
    const policyJson = (values.policy_json || '').trim();
    const payload = {
      id: currentChannel.id || 0,
      platform: values.platform,
      name: values.name,
      app_id: values.app_id,
      org_id: values.org_id || '',
      // 掩码原样提交：后端收到掩码或空串表示沿用库中真值（R84/R102）
      app_secret: values.app_secret || '',
      enabled: values.enabled,
      chat_enabled: values.chat_enabled,
      agent_id: values.chat_enabled === 1 ? values.agent_id || 0 : 0,
      // 空串按空对象提交，与「全部拒绝」语义一致（R102）
      policy_json: policyJson === '' ? '{}' : policyJson,
      revision: currentChannel.revision || 0,
    };
    const res = await channelSave(payload);
    if (res.code !== 0) {
      message.error(res.msg);
      // 乐观锁冲突没有独立错误码：重拉详情，只有 revision 确实变了才刷新表单，
      // 避免普通校验失败（如密钥为空）也把用户输入冲掉。
      if (payload.id > 0) {
        const detail = await channelDetail({ id: payload.id });
        if (detail.code === 0 && detail.data) {
          const fresh = detail.data;
          if (fresh.revision !== currentChannel.revision) {
            setCurrentChannel(fresh);
            formRef.current?.setFieldsValue({
              platform: fresh.platform,
              name: fresh.name,
              app_id: fresh.app_id,
              org_id: fresh.org_id,
              app_secret: fresh.app_secret,
              enabled: fresh.enabled,
              chat_enabled: fresh.chat_enabled,
              agent_id: fresh.agent_id || undefined,
              policy_json: fresh.policy_json,
            });
            message.warning(
              intl.formatMessage({ id: 'plugin.channels.save.stale' }),
            );
          }
        }
      }
      return false;
    }
    const data = res.data || {};
    if (data.connection_pending) {
      message.warning(intl.formatMessage({ id: 'plugin.channels.save.pending' }));
    } else {
      message.success(res.msg);
    }
    setEditVisible(false);
    actionRef.current?.reload();
    return true;
  };

  const handleDelete = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.channels.delete.confirm' }),
      content: intl.formatMessage({ id: 'plugin.channels.delete.content' }),
      okButtonProps: { danger: true },
      onOk: async () => {
        const res = await channelDelete({ id: record.id });
        if (res.code !== 0) {
          message.error(res.msg);
          return;
        }
        message.success(res.msg);
        actionRef.current?.reload();
      },
    });
  };

  const openStatus = async (record: any) => {
    setStatusChannel(record);
    setStatusData(null);
    setStatusVisible(true);
    setStatusLoading(true);
    const res = await channelStatus({ id: record.id });
    setStatusLoading(false);
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    setStatusData(res.data || {});
  };

  const openTest = async (record: any) => {
    setTestChannel(record);
    setTestOptions([]);
    setTestVisible(true);
    const res = await channelTargets({ channel_id: record.id });
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    const list = Array.isArray(res.data) ? res.data : [];
    setTestOptions(
      list
        .filter((item: any) => item.approved === 1)
        .map((item: any) => ({
          label: `${item.platform_target_id} (${item.kind})`,
          value: item.id,
        })),
    );
  };

  // 试发与只校验共用同一份结果提示：按 class 区分凭据/权限问题与平台暂时不可用，
  // message 来自后端且已脱敏，原样展示
  const showTestResult = (data: any) => {
    if (data.class === 'permanent') {
      Modal.error({
        title: intl.formatMessage({ id: 'plugin.channels.test.failed' }),
        content: (
          <>
            <p>
              {intl.formatMessage({ id: 'plugin.channels.test.permanent' })}
            </p>
            {data.message ? <p>{data.message}</p> : null}
          </>
        ),
      });
    } else if (data.class === 'transient') {
      Modal.warning({
        title: intl.formatMessage({ id: 'plugin.channels.test.failed' }),
        content: (
          <>
            <p>
              {intl.formatMessage({ id: 'plugin.channels.test.transient' })}
            </p>
            {data.message ? <p>{data.message}</p> : null}
          </>
        ),
      });
    } else if (data.sent) {
      message.success(intl.formatMessage({ id: 'plugin.channels.test.sent' }));
    } else {
      message.success(intl.formatMessage({ id: 'plugin.channels.test.valid' }));
    }
  };

  // 只校验配置：不带 target_id，后端只检查凭据与适配器可用性
  const handleCheck = async (record: any) => {
    const res = await channelTest({ id: record.id });
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    showTestResult(res.data || {});
  };

  const onSubmitTest = async (values: any) => {
    const res = await channelTest({
      id: testChannel.id,
      target_id: values.target_id || 0,
    });
    if (res.code !== 0) {
      message.error(res.msg);
      return false;
    }
    showTestResult(res.data || {});
    setTestVisible(false);
    return true;
  };

  const loadCapabilities = async (agentId?: number) => {
    setCapLoading(true);
    const res = await channelCapabilities(agentId ? { agent_id: agentId } : {});
    setCapLoading(false);
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    setCapData(res.data || { intents: [], blocked_builtins: [] });
  };

  const openCapabilities = (record: any) => {
    setCapAgentId(record.agent_id || undefined);
    setCapData({ intents: [], blocked_builtins: [] });
    setCapVisible(true);
    loadCapabilities(record.agent_id || undefined);
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.platform' }),
      dataIndex: 'platform',
      render: (_, record) => <Tag>{platformLabel(record.platform)}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.enabled' }),
      dataIndex: 'enabled',
      render: (_, record) =>
        record.enabled === 1 ? (
          <Tag color="green">
            {intl.formatMessage({ id: 'plugin.channels.yes' })}
          </Tag>
        ) : (
          <Tag>{intl.formatMessage({ id: 'plugin.channels.no' })}</Tag>
        ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.chat' }),
      dataIndex: 'chat_enabled',
      render: (_, record) =>
        record.chat_enabled === 1 ? (
          <Tag color="blue">
            {intl.formatMessage({ id: 'plugin.channels.yes' })}
          </Tag>
        ) : (
          <Tag>{intl.formatMessage({ id: 'plugin.channels.no' })}</Tag>
        ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.agent' }),
      dataIndex: 'agent_id',
      render: (_, record) =>
        record.chat_enabled === 1 ? agentName(record.agent_id) : '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.status' }),
      dataIndex: 'status',
      // 状态按渠道单独查询（N 行不做 N 次请求），这里只作为入口
      render: (_, record) => (
        <Button type="link" onClick={() => openStatus(record)}>
          {intl.formatMessage({ id: 'plugin.channels.action.status' })}
        </Button>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.action' }),
      dataIndex: 'action',
      render: (_, record) => (
        <Space size={4} wrap>
          <Button type="link" size="small" onClick={() => openEdit(record)}>
            {intl.formatMessage({ id: 'plugin.channels.action.edit' })}
          </Button>
          <Button type="link" size="small" onClick={() => handleCheck(record)}>
            {intl.formatMessage({ id: 'plugin.channels.action.check' })}
          </Button>
          <Button type="link" size="small" onClick={() => openTest(record)}>
            {intl.formatMessage({ id: 'plugin.channels.action.test' })}
          </Button>
          <Button
            type="link"
            size="small"
            onClick={() => openCapabilities(record)}
          >
            {intl.formatMessage({ id: 'plugin.channels.action.capabilities' })}
          </Button>
          <Button type="link" size="small" danger onClick={() => handleDelete(record)}>
            {intl.formatMessage({ id: 'plugin.channels.action.delete' })}
          </Button>
        </Space>
      ),
    },
  ];

  // 能力抽屉只读：展示意图、风险、动作、菜单映射、拦截原因与当前策略，
  // 不展示任何参数 schema 或密钥
  const capColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.capabilities.intent' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({
        id: 'plugin.channels.capabilities.display-name',
      }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.capabilities.risk' }),
      dataIndex: 'risk',
      render: (_, record) => <Tag>{record.risk}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.capabilities.domain' }),
      dataIndex: 'domain',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.capabilities.actions' }),
      dataIndex: 'actions',
      render: (_, record) =>
        (record.actions || []).length > 0
          ? (record.actions || []).map((item: string) => (
              <Tag key={item}>{item}</Tag>
            ))
          : '-',
    },
    {
      title: intl.formatMessage({
        id: 'plugin.channels.capabilities.menu-paths',
      }),
      dataIndex: 'menu_paths',
      render: (_, record) =>
        (record.menu_paths || []).length > 0
          ? (record.menu_paths || []).map((item: string) => (
              <Tag key={item}>{item}</Tag>
            ))
          : '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.capabilities.blocked' }),
      dataIndex: 'blocked',
      render: (_, record) =>
        record.blocked ? (
          <Space direction="vertical" size={0}>
            <Tag color="red">
              {intl.formatMessage({ id: 'plugin.channels.yes' })}
            </Tag>
            <span>{record.reason}</span>
          </Space>
        ) : (
          intl.formatMessage({ id: 'plugin.channels.no' })
        ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.capabilities.policy' }),
      dataIndex: 'policy',
      render: (_, record) => <Tag>{record.policy}</Tag>,
    },
  ];

  const initialValues = {
    platform: currentChannel.platform || 'feishu',
    name: currentChannel.name || '',
    app_id: currentChannel.app_id || '',
    org_id: currentChannel.org_id || '',
    app_secret: currentChannel.app_secret || '',
    enabled: currentChannel.enabled || 0,
    chat_enabled: currentChannel.chat_enabled || 0,
    agent_id: currentChannel.agent_id || undefined,
    policy_json: currentChannel.policy_json || '',
  };

  return (
    <>
      <ProTable<any>
        rowKey="id"
        search={false}
        actionRef={actionRef}
        pagination={false}
        toolBarRender={() => [
          <Button key="add" type="primary" onClick={() => openCreate()}>
            {intl.formatMessage({ id: 'plugin.channels.add' })}
          </Button>,
        ]}
        request={async () => {
          const res = await channelList();
          if (res.code !== 0) {
            message.error(res.msg);
            return { data: [], success: false, total: 0 };
          }
          const list = Array.isArray(res.data) ? res.data : [];
          return { data: list, success: true, total: list.length };
        }}
        columns={columns}
      />
      {editVisible && (
        <ModalForm
          width={640}
          title={
            currentChannel.id
              ? intl.formatMessage({ id: 'plugin.channels.edit' })
              : intl.formatMessage({ id: 'plugin.channels.add' })
          }
          open={editVisible}
          formRef={formRef}
          initialValues={initialValues}
          layout="horizontal"
          labelCol={{ span: 6 }}
          onOpenChange={(flag) => {
            setEditVisible(flag);
          }}
          onFinish={onSubmitChannel}
        >
          <ProFormSelect
            name="platform"
            label={intl.formatMessage({ id: 'plugin.channels.platform' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.channels.platform.feishu',
                }),
                value: 'feishu',
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.channels.platform.dingtalk',
                }),
                value: 'dingtalk',
              },
            ]}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.field.required',
                }),
              },
            ]}
          />
          <ProFormText
            name="name"
            label={intl.formatMessage({ id: 'plugin.channels.name' })}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.field.required',
                }),
              },
            ]}
          />
          <ProFormText
            name="app_id"
            label={intl.formatMessage({ id: 'plugin.channels.app-id' })}
            extra={intl.formatMessage({
              id: 'plugin.channels.app-id.description',
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
          <ProFormText
            name="org_id"
            label={intl.formatMessage({ id: 'plugin.channels.org-id' })}
            extra={intl.formatMessage({
              id: 'plugin.channels.org-id.description',
            })}
          />
          <ProFormText.Password
            name="app_secret"
            label={intl.formatMessage({ id: 'plugin.channels.app-secret' })}
            extra={intl.formatMessage({
              id: 'plugin.channels.app-secret.description',
            })}
            rules={
              currentChannel.id
                ? []
                : [
                    {
                      required: true,
                      message: intl.formatMessage({
                        id: 'plugin.channels.field.required',
                      }),
                    },
                  ]
            }
          />
          <ProFormRadio.Group
            name="enabled"
            label={intl.formatMessage({ id: 'plugin.channels.enabled' })}
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
            name="chat_enabled"
            label={intl.formatMessage({ id: 'plugin.channels.chat-enabled' })}
            extra={intl.formatMessage({
              id: 'plugin.channels.chat-enabled.description',
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
          <ProFormDependency name={['chat_enabled']}>
            {({ chat_enabled }) => (
              <ProFormSelect
                name="agent_id"
                label={intl.formatMessage({ id: 'plugin.channels.agent' })}
                options={agentOptions}
                showSearch
                allowClear
                placeholder={intl.formatMessage({
                  id: 'plugin.channels.agent.placeholder',
                })}
                extra={
                  agentOptions.length === 0
                    ? intl.formatMessage({ id: 'plugin.channels.agent.empty' })
                    : undefined
                }
                rules={
                  chat_enabled === 1
                    ? [
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'plugin.channels.agent.required',
                          }),
                        },
                      ]
                    : []
                }
              />
            )}
          </ProFormDependency>
          <ProFormTextArea
            name="policy_json"
            label={intl.formatMessage({ id: 'plugin.channels.policy-json' })}
            extra={intl.formatMessage({
              id: 'plugin.channels.policy-json.description',
            })}
            fieldProps={{ rows: 5 }}
            rules={[
              {
                validator: async (_, value: string) => {
                  if (!value || !String(value).trim()) {
                    return;
                  }
                  let parsed: any = null;
                  try {
                    parsed = JSON.parse(value);
                  } catch (e) {
                    throw new Error(
                      intl.formatMessage({
                        id: 'plugin.channels.policy-json.invalid',
                      }),
                    );
                  }
                  if (
                    parsed === null ||
                    typeof parsed !== 'object' ||
                    Array.isArray(parsed)
                  ) {
                    throw new Error(
                      intl.formatMessage({
                        id: 'plugin.channels.policy-json.invalid',
                      }),
                    );
                  }
                },
              },
            ]}
          />
        </ModalForm>
      )}
      <Modal
        open={statusVisible}
        title={intl.formatMessage({ id: 'plugin.channels.status.title' })}
        footer={[
          <Button key="refresh" onClick={() => openStatus(statusChannel)}>
            {intl.formatMessage({ id: 'plugin.channels.refresh' })}
          </Button>,
          <Button
            key="close"
            type="primary"
            onClick={() => setStatusVisible(false)}
          >
            {intl.formatMessage({ id: 'plugin.channels.close' })}
          </Button>,
        ]}
        onCancel={() => setStatusVisible(false)}
      >
        {statusLoading ? (
          <Spin />
        ) : (
          <Descriptions column={1}>
            <Descriptions.Item
              label={intl.formatMessage({ id: 'plugin.channels.table.name' })}
            >
              {statusData?.name || '-'}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({ id: 'plugin.channels.table.status' })}
            >
              {statusData?.connected ? (
                <Tag color="green">
                  {intl.formatMessage({
                    id: 'plugin.channels.status.connected',
                  })}
                </Tag>
              ) : (
                <Tag>
                  {intl.formatMessage({
                    id: 'plugin.channels.status.disconnected',
                  })}
                </Tag>
              )}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({ id: 'plugin.channels.status.lease' })}
            >
              {timeText(statusData?.lease_expires_at)}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'plugin.channels.status.pending',
              })}
            >
              {statusData?.pending_deliveries || 0}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({ id: 'plugin.channels.status.failed' })}
            >
              {statusData?.failed_deliveries || 0}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
      {testVisible && (
        <ModalForm
          width={560}
          title={intl.formatMessage({ id: 'plugin.channels.test.title' })}
          open={testVisible}
          layout="horizontal"
          labelCol={{ span: 6 }}
          onOpenChange={(flag) => {
            setTestVisible(flag);
          }}
          onFinish={onSubmitTest}
        >
          <ProFormSelect
            name="target_id"
            label={intl.formatMessage({ id: 'plugin.channels.test.target' })}
            options={testOptions}
            allowClear
            showSearch
            extra={intl.formatMessage({
              id: 'plugin.channels.test.target.description',
            })}
          />
        </ModalForm>
      )}
      <Drawer
        width={960}
        open={capVisible}
        title={intl.formatMessage({ id: 'plugin.channels.capabilities.title' })}
        onClose={() => setCapVisible(false)}
        extra={
          <Space>
            <span>
              {intl.formatMessage({ id: 'plugin.channels.capabilities.agent' })}
            </span>
            <Select
              style={{ width: 200 }}
              allowClear
              value={capAgentId}
              placeholder={intl.formatMessage({
                id: 'plugin.channels.capabilities.agent.all',
              })}
              options={agentOptions}
              showSearch
              optionFilterProp="label"
              onChange={(value: number) => {
                setCapAgentId(value);
                loadCapabilities(value);
              }}
            />
          </Space>
        }
      >
        {(capData.blocked_builtins || []).length > 0 && (
          <Alert
            style={{ marginBottom: 12 }}
            type="warning"
            message={intl.formatMessage({
              id: 'plugin.channels.capabilities.blocked-builtins',
            })}
            description={
              <Space size={4} wrap>
                {(capData.blocked_builtins || []).map((item: string) => (
                  <Tag key={item}>{item}</Tag>
                ))}
              </Space>
            }
          />
        )}
        <ProTable<any>
          rowKey="name"
          search={false}
          options={false}
          loading={capLoading}
          dataSource={capData.intents || []}
          columns={capColumns}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          size="small"
        />
      </Drawer>
    </>
  );
};

export default ChannelConnect;
