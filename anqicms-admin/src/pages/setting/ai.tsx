import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  checkOpenAIApi,
  getAdminList,
  getSettingAi,
  saveSettingAi,
} from '@/services';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  ModalForm,
  ProForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Col,
  Input,
  List,
  message,
  Row,
  Space,
  Tag,
  Tooltip,
} from 'antd';
import React, { useEffect, useMemo, useRef, useState } from 'react';
import './index.less';

/**
 * 由站点名称推导 MCP server 名。
 *
 * mcpServers 的 key 会被第三方客户端当作服务标识解析（部分客户端不接受空格、
 * 引号等字符），而站点名称是用户自由输入的，因此这里做一次保守清洗：
 * 去掉引号/反斜杠/控制字符，连续空白折成一个短横线，收敛连续与首尾短横线。
 * 清洗后为空（站点名为空或全是非法字符）时回退到 anqicms，保证配置始终可用。
 */
export const buildMcpServerName = (siteName?: string): string => {
  const name = (siteName || '')
    .trim()
    .replace(/[\u0000-\u001f\u007f"'`\\]+/g, '')
    .replace(/\s+/g, '-')
    .replace(/-{2,}/g, '-')
    .replace(/^-+|-+$/g, '');
  return name || 'anqicms';
};

const SettingAiFrom: React.FC<any> = () => {
  const { initialState } = useModel('@@initialState');
  const { isVip, checkVip, VipModal } = useVipModal();
  const [fetched, setFetched] = useState<boolean>(false);
  const [writeSetting, setWriteSetting] = useState<any>({});
  const [chatSetting, setChatSetting] = useState<any>([]);
  const [mcpSetting, setMcpSetting] = useState<any>({
    enabled: false,
    token: '',
    rate_limit: 0,
    exposed_intents: [],
  });
  const [mcpTools, setMcpTools] = useState<any[]>([]);
  const mcpFormRef = useRef<ProFormInstance<any>>();
  const [editChatIndex, setEditChatIndex] = useState<number>(-1);
  const [editChatSetting, setEditChatSetting] = useState<any>({});
  const [editChatOpen, setEditChatOpen] = useState<boolean>(false);
  const [aiEngine, setAiEngine] = useState<string>('');
  const [tmpInput, setTmpInput] = useState<any>({});
  const [newKey, setNewKey] = useState<string>('');
  const [activeTabKey, setActiveTabKey] = useState<string>('mcp');
  // 已访问过的 tab 常驻挂载（仅切换显示），避免切换 tab 时表单被卸载丢失未保存输入
  const [visitedTabs, setVisitedTabs] = useState<string[]>(['mcp']);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingAi();
    let setting = res.data || {};
    setAiEngine(setting?.write?.ai_engine || '');
    setWriteSetting(setting.write || {});
    setChatSetting(setting.chat || []);
    if (setting.mcp) {
      setMcpSetting(setting.mcp);
    }
    setMcpTools(Array.isArray(setting.mcp_tools) ? setting.mcp_tools : []);
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleChangeAiEngine = (e: any) => {
    setAiEngine(e.target.value);
  };

  const handleCheckOpenAIApi = () => {
    const hide = message.loading(
      intl.formatMessage({ id: 'plugin.aigenerate.checking' }),
      0,
    );
    checkOpenAIApi()
      .then((res) => {
        if (res.code === 0) {
          message.success(res.msg);
          writeSetting.api_valid = true;
        } else {
          message.error(res.msg);
          writeSetting.api_valid = false;
        }
        setWriteSetting({ ...writeSetting });
      })
      .finally(() => {
        hide();
      });
  };

  const handleRemoveOpenAIKey = (index: number) => {
    writeSetting.open_ai_keys?.splice(index, 1);
    setWriteSetting({ ...writeSetting });
  };

  const handleAddOpenAIKey = () => {
    if (!tmpInput['key']) {
      return;
    }
    if (!writeSetting.open_ai_keys) {
      writeSetting.open_ai_keys = [];
    }
    let exists = false;
    for (const item of writeSetting.open_ai_keys) {
      if (item.key === tmpInput['key']) {
        exists = true;
        break;
      }
    }
    if (!exists) {
      writeSetting.open_ai_keys.push({
        key: tmpInput['key'],
        invalid: false,
      });

      tmpInput['key'] = '';
    }
    setWriteSetting({ ...writeSetting });
  };

  const handleChangeTmpInput = (field: string, e: any) => {
    tmpInput[field] = e.target.value;
    setTmpInput({ ...tmpInput });
  };

  const onSubmitWrite = async (values: any) => {
    const postData = Object.assign(writeSetting, values);

    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingAi({ write: postData })
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const onAddChatAi = () => {
    setEditChatIndex(-1);
    setEditChatSetting({
      name: '',
      base_url: '',
      api_key: '',
      model: '',
      enable_reasoning: true,
      max_tokens: 8192,
    });
    setEditChatOpen(true);
  };

  const onEditChatSetting = (index: number, item: any) => {
    checkVip(() => {
      setEditChatIndex(index);
      setEditChatSetting(
        item || {
          name: '',
          base_url: '',
          api_key: '',
          model: '',
          enable_reasoning: true,
          max_tokens: 8192,
        },
      );
      setEditChatOpen(true);
    });
  };

  const handleDeleteChatSetting = (index: number) => {
    chatSetting.splice(index, 1);
    saveSettingAi({ chat: chatSetting })
      .then((res: any) => {
        if (res.code === 0) {
          if (Array.isArray(res.data)) {
            setChatSetting(res.data);
          }
          setEditChatSetting(null);
          message.success(
            intl.formatMessage({ id: 'setting.ai.delete-success' }),
          );
        } else {
          message.info(
            res.msg || intl.formatMessage({ id: 'setting.ai.delete-failed' }),
          );
        }
      })
      .catch(() =>
        message.error(intl.formatMessage({ id: 'setting.ai.delete-success' })),
      );
  };

  const onSubmitChat = async (values: any) => {
    if (!values.name || !values.base_url || !values.api_key || !values.model) {
      message.warning(
        intl.formatMessage({ id: 'setting.ai.fill-endpoint-info' }),
      );
      return;
    }
    const provider = Object.assign({}, editChatSetting, values);
    if (editChatIndex === -1) {
      chatSetting.push(provider);
    } else {
      chatSetting[editChatIndex] = provider;
    }

    saveSettingAi({ chat: chatSetting })
      .then((res: any) => {
        if (res.code === 0) {
          if (Array.isArray(res.data.chat)) setChatSetting(res.data.chat);
          setEditChatOpen(false);
          setEditChatSetting(null);
          message.success(
            intl.formatMessage({ id: 'setting.ai.save-success' }),
          );
        } else {
          message.info(
            res.msg || intl.formatMessage({ id: 'setting.ai.save-failed' }),
          );
        }
      })
      .catch(() =>
        message.error(intl.formatMessage({ id: 'setting.ai.save-failed' })),
      );
  };

  const onSubmitMcp = async (values: any) => {
    const postData = {
      ...mcpSetting,
      ...values,
    };
    // exposed_intents：逗号/空白分隔字符串转数组
    if (typeof postData.exposed_intents === 'string') {
      postData.exposed_intents = postData.exposed_intents
        .split(',')
        .map((s: string) => s.trim())
        .filter(Boolean);
    } else if (!Array.isArray(postData.exposed_intents)) {
      postData.exposed_intents = [];
    }
    // api_exposure 的三个名单同理：表单里是逗号分隔字符串，后端要数组
    const exposure = {
      ...(mcpSetting.api_exposure || {}),
      ...(postData.api_exposure || {}),
    };
    for (const k of ['allow_ns', 'deny_ns', 'deny_endpoints']) {
      if (typeof exposure[k] === 'string') {
        exposure[k] = exposure[k]
          .split(',')
          .map((s: string) => s.trim())
          .filter(Boolean);
      } else if (!Array.isArray(exposure[k])) {
        exposure[k] = [];
      }
    }
    // 必填项：mode 缺失时按前端默认 read_write 兜底（后端仍对空值 fail-closed）
    exposure.mode = exposure.mode || 'read_write';
    postData.api_exposure = exposure;
    if (typeof postData.invoke_admin_id !== 'number') {
      postData.invoke_admin_id = Number(postData.invoke_admin_id) || 0;
    }

    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingAi({ mcp: postData })
      .then((res: any) => {
        if (res.code === 0) {
          if (res.data?.mcp) {
            setMcpSetting(res.data.mcp);
          }
          message.success(res.msg);
        } else {
          message.info(
            res.msg || intl.formatMessage({ id: 'setting.ai.save-failed' }),
          );
        }
      })
      .catch(() =>
        message.error(intl.formatMessage({ id: 'setting.ai.save-failed' })),
      )
      .finally(() => {
        hide();
      });
  };

  const handleGenerateToken = () => {
    // 生成 32 位随机 token
    const chars =
      'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let token = '';
    for (let i = 0; i < 32; i++) {
      token += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    // 直接写入 ProForm 字段，避免表单内部值与 state 不同步
    mcpFormRef.current?.setFieldValue('token', token);
    setMcpSetting({ ...mcpSetting, token });
  };

  const handleAddIntent = (name: string) => {
    const field = mcpFormRef.current as any;
    const cur: string = field?.getFieldValue?.('exposed_intents') || '';
    const parts = cur
      .split(',')
      .map((s: string) => s.trim())
      .filter(Boolean);
    if (parts.includes(name)) {
      return;
    }
    parts.push(name);
    mcpFormRef.current?.setFieldValue('exposed_intents', parts.join(', '));
  };

  // 按能力域分组，用于「暴露的工具列表」下方的可选清单
  const toolsByDomain = mcpTools.reduce((acc: any, t: any) => {
    const d = t.domain || 'other';
    (acc[d] = acc[d] || []).push(t);
    return acc;
  }, {});

  // 复制配置里的 server 名跟随站点名称，站点改名后重新复制即为新名字
  const mcpServerName = useMemo(
    () => buildMcpServerName(initialState?.system?.site_name),
    [initialState?.system?.site_name],
  );

  const handleCopyMcpConfig = () => {
    const baseUrl = initialState?.system?.base_url || window.location.origin;
    const token = mcpSetting.token || '';
    if (!token) {
      message.warning(
        intl.formatMessage({ id: 'setting.ai.token-generate-first' }),
      );
      return;
    }
    const config = {
      mcpServers: {
        [mcpServerName]: {
          url: `${baseUrl}/api/mcp`,
          headers: {
            Authorization: `Bearer ${token}`,
          },
        },
      },
    };
    const text = JSON.stringify(config, null, 2);
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(
        () =>
          message.success(
            intl.formatMessage({ id: 'setting.ai.copied-to-clipboard' }),
          ),
        () =>
          message.error(
            intl.formatMessage({ id: 'setting.ai.copy-failed-manual' }),
          ),
      );
    } else {
      // 降级方案
      const textarea = document.createElement('textarea');
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();
      try {
        document.execCommand('copy');
        message.success(
          intl.formatMessage({ id: 'setting.ai.copied-to-clipboard' }),
        );
      } catch {
        message.error(
          intl.formatMessage({ id: 'setting.ai.copy-failed-manual' }),
        );
      }
      document.body.removeChild(textarea);
    }
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
        activeTabKey={activeTabKey}
        onTabChange={(tabKey) => {
          setActiveTabKey(tabKey);
          setVisitedTabs((prev) =>
            prev.includes(tabKey) ? prev : [...prev, tabKey],
          );
        }}
        tabList={[
          {
            key: 'write',
            label: intl.formatMessage({ id: 'setting.tab.ai-writing' }),
          },
          {
            key: 'chat',
            label: intl.formatMessage({ id: 'setting.tab.ai-assistant' }),
          },
          {
            key: 'mcp',
            label: intl.formatMessage({ id: 'setting.tab.mcp' }),
          },
        ]}
      >
        {fetched && visitedTabs.includes('write') && (
          <div style={{ display: activeTabKey === 'write' ? 'block' : 'none' }}>
          <ProForm initialValues={writeSetting} onFinish={onSubmitWrite}>
              <ProFormRadio.Group
                name="ai_engine"
                label={intl.formatMessage({
                  id: 'plugin.aigenerate.source',
                })}
                options={[
                  {
                    label: intl.formatMessage({
                      id: 'plugin.aigenerate.source.anqicms',
                    }),
                    value: '',
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.aigenerate.source.openai',
                    }),
                    value: 'openai',
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.aigenerate.source.deepseek',
                    }),
                    value: 'deepseek',
                  },
                  {
                    label: intl.formatMessage({
                      id: 'plugin.aigenerate.source.spark',
                    }),
                    value: 'spark',
                  },
                ]}
                fieldProps={{
                  onChange: (e) => {
                    handleChangeAiEngine(e);
                  },
                }}
                extra={
                  <div>
                    <span>
                      <FormattedMessage id="plugin.aigenerate.source.description" />
                    </span>
                    <Tag
                      style={{ marginLeft: 10 }}
                      className="link"
                      onClick={handleCheckOpenAIApi}
                    >
                      <FormattedMessage id="plugin.aigenerate.source.check-openai" />
                    </Tag>
                  </div>
                }
                disabled={isVip === false}
              />
              {!isVip ? (
                <div
                  className="link mb-normal"
                  onClick={() => {
                    checkVip(() => {});
                  }}
                >
                  <FormattedMessage id="setting.ai.vip-more-ai" />
                </div>
              ) : null}
              {(aiEngine === 'openai' || aiEngine === 'deepseek') && (
                <>
                  <ProFormText
                    name={'open_ai_api'}
                    label={intl.formatMessage({
                      id: 'plugin.aigenerate.openai.base-url',
                    })}
                    extra={intl.formatMessage({
                      id:
                        aiEngine === 'deepseek'
                          ? 'plugin.aigenerate.openai.base-url.deepseek'
                          : 'plugin.aigenerate.openai.base-url.openai',
                    })}
                  />
                  <ProFormText
                    name={'open_ai_model'}
                    label={intl.formatMessage({
                      id: 'plugin.aigenerate.openai.model',
                    })}
                    extra={intl.formatMessage({
                      id:
                        aiEngine === 'deepseek'
                          ? 'plugin.aigenerate.openai.model.deepseek'
                          : 'plugin.aigenerate.openai.model.openai',
                    })}
                  />
                  <ProFormText
                    label="API Keys"
                    extra={
                      <div>
                        <div className="text-muted">
                          <div>
                            <span className="text-red">*</span>
                            <FormattedMessage id="plugin.aigenerate.openai.description" />
                          </div>
                        </div>
                        <div className="tag-lists">
                          <Space size={[12, 12]} wrap>
                            {writeSetting.open_ai_keys?.map(
                              (tag: any, index: number) => (
                                <span className="edit-tag" key={index}>
                                  <span className="key">{tag.key}</span>
                                  <span className="divide">
                                    <span className="value">
                                      {tag.invalid
                                        ? intl.formatMessage({
                                            id: 'plugin.aigenerate.openai.invalid',
                                          })
                                        : intl.formatMessage({
                                            id: 'plugin.aigenerate.openai.valid',
                                          })}
                                    </span>
                                  </span>
                                  <span
                                    className="close"
                                    onClick={() => handleRemoveOpenAIKey(index)}
                                  >
                                    ×
                                  </span>
                                </span>
                              ),
                            )}
                          </Space>
                        </div>
                      </div>
                    }
                  >
                    <Input.Group compact>
                      <Input
                        value={tmpInput.key || ''}
                        onChange={(e) => handleChangeTmpInput('key', e)}
                        onPressEnter={() => handleAddOpenAIKey()}
                        suffix={
                          <a onClick={() => handleAddOpenAIKey()}>
                            <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                          </a>
                        }
                      />
                    </Input.Group>
                  </ProFormText>
                </>
              )}
              {aiEngine === 'spark' && (
                <>
                  <div className="mb-normal">
                    <FormattedMessage id="plugin.aigenerate.spark.description" />
                    :
                    <a
                      href="https://xinghuo.xfyun.cn/sparkapi?ch=gjp"
                      target="_blank"
                      rel="noreferrer"
                    >
                      https://xinghuo.xfyun.cn/sparkapi?ch=gjp
                    </a>
                  </div>
                  <ProFormRadio.Group
                    name={['spark', 'version']}
                    label={intl.formatMessage({
                      id: 'plugin.aigenerate.spark.version',
                    })}
                    options={[
                      { label: 'Spark Lite(Free)', value: '1.5' },
                      { label: 'Spark Pro', value: '3.0' },
                      { label: 'Spark Max', value: '3.5' },
                      { label: 'Spark4.0 Ultra', value: '4.0' },
                    ]}
                  />
                  <ProFormText name={['spark', 'app_id']} label="APPID" />
                  <ProFormText
                    name={['spark', 'api_secret']}
                    label="APISecret"
                  />
                  <ProFormText name={['spark', 'api_key']} label="APIKey" />
                </>
              )}
            </ProForm>
          </div>
        )}
        {fetched && visitedTabs.includes('chat') && (
          <div style={{ display: activeTabKey === 'chat' ? 'block' : 'none' }}>
            <div>
              <div style={{ marginBottom: 12 }}>
                <Button type="primary" size="small" onClick={onAddChatAi}>
                  <FormattedMessage id="setting.ai.add-custom-endpoint" />
                </Button>
              </div>
              {chatSetting.length === 0 ? (
                <div
                  style={{
                    textAlign: 'center',
                    padding: '24px 0',
                    color: '#999',
                  }}
                >
                  <FormattedMessage id="setting.ai.no-custom-endpoint" />
                </div>
              ) : (
                <List
                  size="small"
                  dataSource={chatSetting}
                  renderItem={(item: any, index: number) => (
                    <List.Item
                      actions={[
                        <Button
                          key="edit"
                          type="link"
                          size="small"
                          onClick={() => onEditChatSetting(index, item)}
                        >
                          <FormattedMessage id="common.edit" />
                        </Button>,
                        <Button
                          key="delete"
                          type="link"
                          size="small"
                          danger
                          onClick={() => handleDeleteChatSetting(index)}
                        >
                          <FormattedMessage id="setting.system.delete" />
                        </Button>,
                      ]}
                    >
                      <List.Item.Meta
                        title={item.name}
                        description={
                          <span style={{ fontSize: 12, color: '#999' }}>
                            {item.base_url} | {item.model}
                          </span>
                        }
                      />
                    </List.Item>
                  )}
                />
              )}
            </div>
          </div>
        )}
        {fetched && visitedTabs.includes('mcp') && (
          <div style={{ display: activeTabKey === 'mcp' ? 'block' : 'none' }}>
          <ProForm
              formRef={mcpFormRef}
              initialValues={{
                enabled: mcpSetting.enabled || false,
                token: mcpSetting.token || '',
                rate_limit: mcpSetting.rate_limit || 0,
                invoke_admin_id: mcpSetting.invoke_admin_id || 0,
                exposed_intents: Array.isArray(mcpSetting.exposed_intents)
                  ? mcpSetting.exposed_intents.join(', ')
                  : '',
                api_exposure: {
                  mode: mcpSetting.api_exposure?.mode || 'read_write',
                  allow_ns: Array.isArray(mcpSetting.api_exposure?.allow_ns)
                    ? mcpSetting.api_exposure.allow_ns.join(', ')
                    : '',
                  deny_ns: Array.isArray(mcpSetting.api_exposure?.deny_ns)
                    ? mcpSetting.api_exposure.deny_ns.join(', ')
                    : '',
                  deny_endpoints: Array.isArray(
                    mcpSetting.api_exposure?.deny_endpoints,
                  )
                    ? mcpSetting.api_exposure.deny_endpoints.join(', ')
                    : '',
                },
              }}
              onFinish={onSubmitMcp}
            >
              <div
                style={{
                  marginBottom: 16,
                  padding: 12,
                  background: '#f6f8fa',
                  borderRadius: 6,
                  fontSize: 13,
                  color: '#666',
                }}
              >
                {intl.formatMessage(
                  { id: 'setting.ai.mcp-intro' },
                  {
                    tools:
                      mcpTools.length > 0
                        ? intl.formatMessage(
                            { id: 'setting.ai.mcp-tools-count' },
                            { count: mcpTools.length },
                          )
                        : intl.formatMessage({
                            id: 'setting.ai.mcp-tools-all',
                          }),
                  },
                )}
                <div style={{ marginTop: 4 }}>
                  {intl.formatMessage(
                    { id: 'setting.ai.mcp-server-name' },
                    { name: mcpServerName },
                  )}
                  <code style={{ marginLeft: 6 }}>{mcpServerName}</code>
                </div>
                <code style={{ marginLeft: 6 }}>
                  {initialState?.system?.base_url || ''}/api/mcp
                </code>
                <Button
                  type="link"
                  size="small"
                  onClick={handleCopyMcpConfig}
                  style={{ float: 'right', padding: 0 }}
                >
                  <FormattedMessage id="setting.ai.copy-mcp-config" />
                </Button>
              </div>
              <Row gutter={24}>
                <Col xs={24} md={12}>
                  <ProFormSwitch
                    name="enabled"
                    label={intl.formatMessage({ id: 'setting.ai.mcp-enable' })}
                    extra={intl.formatMessage({
                      id: 'setting.ai.mcp-enable-description',
                    })}
                  />
                  <ProFormText
                    name="token"
                    label={intl.formatMessage({
                      id: 'setting.ai.mcp-token-label',
                    })}
                    placeholder={intl.formatMessage({
                      id: 'setting.ai.mcp-token-placeholder',
                    })}
                    extra={intl.formatMessage({
                      id: 'setting.ai.mcp-token-extra',
                    })}
                    fieldProps={{
                      addonAfter: (
                        <Button size="small" onClick={handleGenerateToken}>
                          <FormattedMessage id="setting.ai.mcp-generate-token" />
                        </Button>
                      ),
                    }}
                  />
                  <ProFormDigit
                    name="rate_limit"
                    label={intl.formatMessage({
                      id: 'setting.ai.mcp-rate-limit',
                    })}
                    placeholder={intl.formatMessage({
                      id: 'setting.ai.mcp-rate-limit-placeholder',
                    })}
                    min={0}
                    extra={intl.formatMessage({
                      id: 'setting.ai.mcp-rate-limit-extra',
                    })}
                  />
                  <div
                    style={{
                      border: '1px solid #ffd591',
                      background: '#fffbe6',
                      borderRadius: 8,
                      padding: '12px 12px 0',
                      marginBottom: 16,
                    }}
                  >
                    <ProFormSelect
                      name="invoke_admin_id"
                      label={intl.formatMessage({
                        id: 'setting.ai.mcp-invoke-admin',
                      })}
                      extra={intl.formatMessage({
                        id: 'setting.ai.mcp-invoke-admin-extra',
                      })}
                      showSearch
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'setting.ai.mcp-invoke-admin-required',
                          }),
                        },
                      ]}
                      fieldProps={{
                        optionFilterProp: 'label',
                      }}
                      request={async () => {
                        const res: any = await getAdminList({
                          page: 1,
                          page_size: 200,
                        });
                        const list =
                          res?.data?.list || res?.data || res?.list || [];
                        // 后端 invoke_admin_id 是 uint，未配置也返回 0，无法与「明确选了不开放」区分。
                        // 因此只在整套 MCP 从未配置过（没有 token）时默认选中第一个管理员，
                        // 让新用户第一次保存即可用；已有 token 则尊重其现有选择。
                        if (
                          !mcpSetting.token &&
                          !mcpSetting.invoke_admin_id &&
                          list.length > 0
                        ) {
                          mcpFormRef.current?.setFieldsValue({
                            invoke_admin_id: list[0].id,
                          });
                        }
                        return [
                          {
                            label: intl.formatMessage({
                              id: 'setting.ai.mcp-invoke-admin-none',
                            }),
                            value: 0,
                          },
                          ...list.map((a: any) => ({
                            label: `${a.user_name}（ID ${a.id}）`,
                            value: a.id,
                          })),
                        ];
                      }}
                    />
                  </div>
                </Col>
                <Col xs={24} md={12}>
                  <Alert
                    type="info"
                    showIcon
                    style={{ marginBottom: 16 }}
                    message={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-mode-attention',
                    })}
                  />
                  <div
                    style={{
                      border: '1px solid #ffd591',
                      background: '#fffbe6',
                      borderRadius: 8,
                      padding: '12px 12px 0',
                      marginBottom: 16,
                    }}
                  >
                    <ProFormSelect
                      name={['api_exposure', 'mode']}
                      label={intl.formatMessage({
                        id: 'setting.ai.mcp-exposure-mode',
                      })}
                      extra={intl.formatMessage({
                        id: 'setting.ai.mcp-exposure-mode-extra',
                      })}
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'setting.ai.mcp-exposure-mode-required',
                          }),
                        },
                      ]}
                      options={[
                        {
                          label: intl.formatMessage({
                            id: 'setting.ai.mcp-exposure-mode-off',
                          }),
                          value: 'off',
                        },
                        {
                          label: intl.formatMessage({
                            id: 'setting.ai.mcp-exposure-mode-read',
                          }),
                          value: 'read',
                        },
                        {
                          label: intl.formatMessage({
                            id: 'setting.ai.mcp-exposure-mode-read-write',
                          }),
                          value: 'read_write',
                        },
                        {
                          label: intl.formatMessage({
                            id: 'setting.ai.mcp-exposure-mode-all',
                          }),
                          value: 'all',
                        },
                      ]}
                    />
                  </div>
                  <ProFormText
                    name={['api_exposure', 'allow_ns']}
                    label={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-allow-ns',
                    })}
                    placeholder={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-allow-ns-placeholder',
                    })}
                    extra={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-allow-ns-extra',
                    })}
                  />
                  <ProFormText
                    name={['api_exposure', 'deny_ns']}
                    label={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-deny-ns',
                    })}
                    extra={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-deny-ns-extra',
                    })}
                  />
                  <ProFormText
                    name={['api_exposure', 'deny_endpoints']}
                    label={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-deny-endpoints',
                    })}
                    placeholder={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-deny-endpoints-placeholder',
                    })}
                    extra={intl.formatMessage({
                      id: 'setting.ai.mcp-exposure-deny-endpoints-extra',
                    })}
                  />
                </Col>
              </Row>
              <ProFormText
                name="exposed_intents"
                label={intl.formatMessage({
                  id: 'setting.ai.mcp-exposed-tools',
                })}
                placeholder={intl.formatMessage({
                  id: 'setting.ai.mcp-exposed-tools-placeholder',
                })}
                extra={intl.formatMessage({
                  id: 'setting.ai.mcp-exposed-tools-extra',
                })}
              />
              {mcpTools.length > 0 && (
                <div
                  style={{
                    marginTop: -8,
                    marginBottom: 16,
                    padding: 12,
                    background: '#fafafa',
                    border: '1px solid #f0f0f0',
                    borderRadius: 6,
                  }}
                >
                  <div
                    style={{
                      fontSize: 13,
                      color: '#999',
                      marginBottom: 8,
                    }}
                  >
                    <FormattedMessage
                      id="setting.ai.mcp-available-tools"
                      values={{ count: mcpTools.length }}
                    />
                  </div>
                  {Object.entries(toolsByDomain).map(([domain, list]: any) => (
                    <div key={domain} style={{ marginBottom: 6 }}>
                      <span
                        style={{
                          fontSize: 12,
                          color: '#666',
                          marginRight: 8,
                        }}
                      >
                        {domain}
                      </span>
                      <Space size={[6, 6]} wrap>
                        {list.map((t: any) => (
                          <Tooltip
                            key={t.name}
                            title={
                              <div>
                                <div>{t.title || t.name}</div>
                                <div style={{ marginTop: 4 }}>{t.desc}</div>
                                <div style={{ marginTop: 4 }}>
                                  <FormattedMessage id="setting.ai.mcp-risk-level" />
                                  {t.risk}
                                  {t.default_off ? (
                                    <FormattedMessage id="setting.ai.mcp-default-off" />
                                  ) : null}
                                </div>
                              </div>
                            }
                          >
                            <Tag
                              style={{
                                cursor: 'pointer',
                                marginRight: 0,
                                opacity: t.default_off ? 0.65 : 1,
                              }}
                              color={
                                t.risk === 'read'
                                  ? 'green'
                                  : t.risk === 'write'
                                  ? 'blue'
                                  : t.risk === 'destructive'
                                  ? 'orange'
                                  : 'red'
                              }
                              onClick={() => handleAddIntent(t.name)}
                            >
                              {t.name}
                            </Tag>
                          </Tooltip>
                        ))}
                      </Space>
                    </div>
                  ))}
                </div>
              )}
            </ProForm>
          </div>
        )}
      </Card>
      <VipModal />
      <ModalForm
        title={
          editChatIndex !== -1
            ? intl.formatMessage({ id: 'setting.ai.edit-custom-endpoint' })
            : intl.formatMessage({ id: 'setting.ai.add-custom-endpoint' })
        }
        open={editChatOpen}
        onOpenChange={(flag) => {
          setEditChatOpen(flag);
        }}
        layout="horizontal"
        initialValues={editChatSetting || {}}
        onFinish={async (values) => {
          onSubmitChat(values);
        }}
        modalProps={{ maskClosable: false }}
        width={520}
      >
        {editChatOpen && (
          <div>
            <ProFormText
              name="name"
              label={intl.formatMessage({ id: 'setting.ai.endpoint-name' })}
              placeholder={intl.formatMessage({
                id: 'setting.ai.endpoint-name-example',
              })}
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'setting.ai.endpoint-name-required',
                  }),
                },
              ]}
            />
            <ProFormText
              name="base_url"
              label={intl.formatMessage({ id: 'setting.ai.api-url' })}
              placeholder="https://api.openai.com/v1"
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'setting.ai.api-url-required',
                  }),
                },
              ]}
            />
            <ProFormText
              name="api_key"
              label="API Key"
              placeholder="sk-xxxxxxxxxxxxxxxx"
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'setting.ai.api-key-required',
                  }),
                },
              ]}
            />
            <ProFormText
              name="model"
              label={intl.formatMessage({ id: 'setting.ai.model' })}
              placeholder="deepseek-v4-flash"
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'setting.ai.model-required',
                  }),
                },
              ]}
            />
            <ProFormRadio.Group
              name="enable_reasoning"
              label={intl.formatMessage({ id: 'setting.ai.thinking-mode' })}
              options={[
                {
                  label: intl.formatMessage({ id: 'setting.ai.thinking-on' }),
                  value: true,
                },
                {
                  label: intl.formatMessage({ id: 'setting.ai.thinking-off' }),
                  value: false,
                },
              ]}
            />
            <ProFormDigit
              name="max_tokens"
              label={intl.formatMessage({ id: 'setting.ai.max-tokens' })}
              placeholder="8192"
            />
            <ProFormDigit
              name="timeout_seconds"
              label={intl.formatMessage({ id: 'setting.ai.timeout-seconds' })}
              placeholder="120"
            />
          </div>
        )}
      </ModalForm>
    </NewContainer>
  );
};

export default SettingAiFrom;
