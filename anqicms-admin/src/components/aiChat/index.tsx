import {
  anqiAiChatTrace,
  anqiAiToolConfirm,
  anqiAiUpload,
  anqiSkillDelete,
  anqiSkillReload,
  getAiAgentLogs,
  getAiAgents,
  getAiHistories,
  getAiSessions,
  getAiSettings,
  getDesignInfo,
  getSkillList,
  saveAiSettings,
} from '@/services';
import config from '@/services/config';
import { getSessionStore, getStore, setStore } from '@/utils/store';
import { CloudServerOutlined, PaperClipOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Drawer, Empty, MenuProps, Modal, Spin, Tag, Upload, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import AgentDrawer from './components/AgentDrawer';
import AgentLogDrawer from './components/AgentLogDrawer';
import ChatInput from './components/ChatInput';
import ChatMessages from './components/ChatMessages';
import Header from './components/Header';
import SettingsPanel, { ProviderFormModal } from './components/SettingsPanel';
import SkillListDrawer from './components/SkillListDrawer';
import TemplateModal from './components/TemplateModal';
import './index.less';
import {
  AiChatProps,
  AiProviderConfig,
  Message,
  Segment,
  TokenUsage,
} from './types';
import { convertHistoryMessages } from './utils';

// 执行链路追踪事件的阶段 / 决策配色（P2-7）
const TRACE_PHASE_COLOR: Record<string, string> = {
  round: 'blue',
  tool: 'green',
  guardrail: 'gold',
  retry: 'purple',
  compact: 'cyan',
  agent: 'geekblue',
  note: 'default',
};
const TRACE_DECISION_COLOR: Record<string, string> = {
  proceed: 'green',
  allow: 'green',
  deny: 'red',
  ask: 'orange',
  block: 'red',
};

// 审批项的风险配色（与后端 intent.Risk 口径一致）
const RISK_COLOR: Record<string, string> = {
  read: 'green',
  write: 'blue',
  destructive: 'orange',
  system: 'red',
};

const APPROVAL_ARGS_PRE_STYLE: CSSProperties = {
  background: '#f5f5f5',
  padding: 12,
  borderRadius: 4,
  maxHeight: 200,
  overflow: 'auto',
  fontSize: 12,
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-all',
  margin: '4px 0',
};

// =====================================================================
// 主组件
// =====================================================================

const AiChat: React.FC<AiChatProps> = ({
  visible,
  onClose,
  iframeUrl,
  selectedDom,
  onIframeReload,
}) => {
  const intl = useIntl();
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputValue, setInputValue] = useState('');
  const [loading, setLoading] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [sessionId, setSessionId] = useState(getStore('aiSessionId') || '');
  const [maximized, setMaximized] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string>('');
  const [sessionList, setSessionList] = useState<any[]>([]);
  const [sessionListOpen, setSessionListOpen] = useState(false);
  const [sessionListLoading, setSessionListLoading] = useState(false);
  const [selectedModel, setSelectedModel] = useState<string>(
    getStore('aiChatModel') || 'deepseek',
  );
  const [uploadedFiles, setUploadedFiles] = useState<any[]>([]);
  const [templateModalVisible, setTemplateModalVisible] = useState(false);
  const [templateFiles, setTemplateFiles] = useState<any[]>([]);
  const [templateLoading, setTemplateLoading] = useState(false);
  const [settingsModalVisible, setSettingsModalVisible] = useState(false);
  const [customProviders, setCustomProviders] = useState<AiProviderConfig[]>(
    [],
  );
  const [editProviderIndex, setEditProviderIndex] = useState(-1);
  const [editProvider, setEditProvider] = useState<AiProviderConfig | null>(
    null,
  );
  const [editProviderModalVisible, setEditProviderModalVisible] =
    useState(false);
  const [tokenUsage, setTokenUsage] = useState<TokenUsage>({
    prompt: 0,
    completion: 0,
    total: 0,
  });
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const welcomeShownRef = useRef(false);

  // ── Agent 管理状态 ──
  const [agentDrawerVisible, setAgentDrawerVisible] = useState(false);
  const [agentList, setAgentList] = useState<any[]>([]);
  const [agentListLoading, setAgentListLoading] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<any>(null);
  const [agentLogs, setAgentLogs] = useState<any[]>([]);
  const [agentLogsVisible, setAgentLogsVisible] = useState(false);
  const [agentLogsLoading, setAgentLogsLoading] = useState(false);

  // ── Skill 管理状态 ──
  const [skillDrawerVisible, setSkillDrawerVisible] = useState(false);
  const [skillList, setSkillList] = useState<any[]>([]);
  const [skillListLoading, setSkillListLoading] = useState(false);

  // ── P2-7: 执行链路追踪面板 ──
  const [traceDrawerVisible, setTraceDrawerVisible] = useState(false);
  const [traceEvents, setTraceEvents] = useState<any[]>([]);
  const [traceLoading, setTraceLoading] = useState(false);

  // ── P0: 工具执行审批 ──
  // 用 antd Modal.confirm 命令式 API（在 tool_confirm handler 内调用），
  // 不再持有 pendingApproval React state，避免渲染队列积压导致 Modal 延迟弹出。

  const handleShowAgentLogs = async (agent: any) => {
    setSelectedAgent(agent);
    setAgentLogsVisible(true);
    setAgentLogsLoading(true);
    try {
      const res = await getAiAgentLogs(agent.id);
      if (Array.isArray(res.data)) setAgentLogs(res.data);
    } catch (_) {}
    setAgentLogsLoading(false);
  };

  // P2-7: 打开执行链路追踪面板并拉取最近事件
  const handleOpenTraceDrawer = async () => {
    setTraceDrawerVisible(true);
    setTraceLoading(true);
    try {
      const res = await anqiAiChatTrace({ limit: 200 });
      if (Array.isArray(res?.data?.events)) {
        setTraceEvents(res.data.events);
      } else {
        setTraceEvents([]);
      }
    } catch (_) {
      setTraceEvents([]);
    }
    setTraceLoading(false);
  };

  const loadProviders = () => {
    getAiSettings()
      .then((res: any) => {
        setCustomProviders(res.data?.configs || []);
      })
      .catch(() => {});
  };

  const handleNewSession = () => {
    setMessages([]);
    setTokenUsage({ prompt: 0, completion: 0, total: 0 });
    setSessionId('');
    setStore('aiSessionId', '');
    welcomeShownRef.current = false;
  };

  const handleLoadSessions = () => {
    if (sessionList.length > 0) {
      setSessionListOpen(true);
      return;
    }
    setSessionListLoading(true);
    getAiSessions()
      .then((res: any) => {
        const list = Array.isArray(res.data) ? res.data : [];
        setSessionList(list);
        setSessionListOpen(true);
      })
      .catch(
        () =>
          message.error(
            intl.formatMessage({ id: 'ai.chat.history-load-failed' }),
          ),
      )
      .finally(() => setSessionListLoading(false));
  };

  const handleSwitchSession = (sid: string) => {
    setSessionListOpen(false);
    setSessionId(sid);
    setStore('aiSessionId', sid);
    setTokenUsage({ prompt: 0, completion: 0, total: 0 });
    setHistoryLoading(true);
    getAiHistories({ session_id: sid })
      .then((res: any) => {
        if (Array.isArray(res.data)) {
          setMessages(convertHistoryMessages(res.data));
        } else {
          setMessages([]);
        }
      })
      .finally(() => setHistoryLoading(false));
  };

  const handleOpenSettings = () => {
    loadProviders();
    setSettingsModalVisible(true);
  };

  // 打开组件时加载历史
  useEffect(() => {
    if (!visible) return;
    if (welcomeShownRef.current) return;
    welcomeShownRef.current = true;

    loadProviders();

    const cachedSessionId = getStore('aiSessionId');
    if (cachedSessionId) {
      setHistoryLoading(true);
      getAiHistories({ session_id: cachedSessionId })
        .then((res) => {
          if (Array.isArray(res.data)) {
            setMessages(convertHistoryMessages(res.data));
          } else {
            setMessages([]);
          }
        })
        .finally(() => setHistoryLoading(false));
    } else {
      setMessages([]);
    }
  }, [visible]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // =====================================================================
  // handleSend — SSE 流式处理
  // =====================================================================
  const handleSend = async (overrideMessage?: string) => {
    const messageContent = (overrideMessage || inputValue).trim();
    if (!messageContent || loading) return;
    setInputValue('');
    setLoading(true);

    const attachedFiles = uploadedFiles.length > 0 ? uploadedFiles : undefined;
    const userMessage: Message = {
      role: 'user',
      segments: [{ type: 'text', content: messageContent }],
      timestamp: Date.now(),
      files: attachedFiles,
    };
    setMessages((prev) => [...prev, userMessage]);
    setErrorMsg('');

    try {
      let adminToken = getStore('adminToken');
      const sessionToken = getSessionStore('adminToken');
      if (sessionToken) adminToken = sessionToken;

      const body: any = {
        session_id: sessionId,
        message: messageContent,
        model: selectedModel,
      };
      // 前端 AI 编辑器：附带 iframe 上下文
      if (iframeUrl) {
        body.iframe_url = iframeUrl;
      }
      if (selectedDom) {
        body.selected_dom = selectedDom;
      }
      if (uploadedFiles.length > 0) {
        body.files = uploadedFiles.map((f: any) => ({
          file_name: f.file_name,
          file_path: f.file_path,
        }));
      }
      setUploadedFiles([]);

      const response = await fetch(config.baseUrl + '/anqi/ai/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Admin: adminToken },
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      // 检查响应类型：如果返回的是 JSON（非 SSE），说明需要特殊处理
      const contentType = response.headers.get('Content-Type') || '';
      if (contentType.includes('application/json')) {
        const jsonResp = await response.json();
        throw new Error(
          jsonResp.msg || intl.formatMessage({ id: 'ai.chat.request-failed' }),
        );
      }

      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error('No readable stream');
      }

      const decoder = new TextDecoder();
      let buffer = '';
      let segments: Segment[] = [];

      // 节流 flushSegments: 用 RAF 合并多个 reasoning/message chunk
      // 避免 React 渲染队列饱和导致电脑卡顿、审批 Modal 延迟弹出
      let flushRafId: number | null = null;
      const flushSegments = () => {
        if (segments.length === 0) return;
        if (flushRafId !== null) return; // 已有待执行的 flush
        flushRafId = requestAnimationFrame(() => {
          flushRafId = null;
          setMessages((prev) => {
            const newMsgs = [...prev];
            const last = newMsgs[newMsgs.length - 1];
            if (last && last.role === 'assistant') {
              newMsgs[newMsgs.length - 1] = {
                ...last,
                segments: [...segments],
              };
            } else {
              newMsgs.push({
                role: 'assistant',
                segments: [...segments],
                timestamp: Date.now(),
              });
            }
            return newMsgs;
          });
        });
      };

      const readStream = async () => {
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const events = buffer.split('\n\n');
            buffer = events.pop() || '';

            for (const event of events) {
              if (!event.trim()) continue;

              const lines = event.split('\n');
              let eventType = 'message';
              let data = '';

              for (const line of lines) {
                if (line.startsWith('event:')) {
                  eventType = line.substring(6).trim();
                } else if (line.startsWith('data:')) {
                  data = line.substring(5).trim();
                }
              }

              if (eventType === 'session' && data) {
                setSessionId(data);
                setStore('aiSessionId', data);
                continue;
              }

              if (eventType === 'config' && data) {
                handleOpenSettings();
                setLoading(false);
                continue;
              }

              // end 事件携带 [DONE] 非 JSON，需在 JSON.parse 前处理
              if (eventType === 'end') {
                // 前端 AI 编辑器：AI 写模板成功后刷新 iframe 预览
                onIframeReload?.();
                continue;
              }

              try {
                const parsed = JSON.parse(data || '{}');

                if (eventType === 'reasoning' && parsed.v !== undefined) {
                  const last = segments[segments.length - 1];
                  if (last && last.type === 'reasoning') {
                    last.content = (last.content || '') + parsed.v;
                  } else {
                    segments.push({ type: 'reasoning', content: parsed.v });
                  }
                  flushSegments();
                  continue;
                }

                if (eventType === 'message' && parsed.v !== undefined) {
                  const last = segments[segments.length - 1];
                  if (last && last.type === 'text') {
                    last.content = (last.content || '') + parsed.v;
                  } else {
                    segments.push({ type: 'text', content: parsed.v });
                  }
                  flushSegments();
                  continue;
                }

                if (eventType === 'tool_call') {
                  console.log('tool_call', parsed);
                  segments.push({
                    type: 'tool_call',
                    toolName: parsed.name || '',
                    toolCallId: parsed.tool_call_id || '',
                    arguments:
                      typeof parsed.arguments === 'object'
                        ? JSON.stringify(parsed.arguments)
                        : parsed.arguments || '',
                    status: 'calling',
                    result: undefined,
                  });
                  flushSegments();
                  continue;
                }

                if (eventType === 'tool_confirm_batch') {
                  console.log('tool_confirm_batch', parsed);
                  // 回合级合并审批：后端先扫完整轮，把所有需要确认的调用
                  // 打包成一次确认，避免并发写操作把弹窗串成一片。
                  const batchId = parsed.tool_call_id || '';
                  const items: any[] = Array.isArray(parsed.items)
                    ? parsed.items
                    : [];
                  const allowFullControl =
                    parsed.allow_full_control === true;

                  const batchResult = async (
                    decision:
                      | 'allow'
                      | 'once_allow'
                      | 'deny'
                      | 'full_control',
                  ) => {
                    await anqiAiToolConfirm({
                      tool_call_id: batchId,
                      decision,
                    });
                  };

                  const batchModal = Modal.confirm({
                    title: intl.formatMessage(
                      { id: 'ai.approval.batch-title' },
                      { count: items.length },
                    ),
                    width: 640,
                    content: (
                      <div>
                        {items.map((it: any, i: number) => {
                          const argsText =
                            typeof it.arguments === 'object'
                              ? JSON.stringify(it.arguments, null, 2)
                              : it.arguments || '';
                          const sensitive = it.path_class === 'sensitive';
                          const pathNote =
                            it.path_class === 'undeterminable' &&
                            !(it.path_targets || []).length
                              ? ''
                              : it.path_class;
                          return (
                            <div
                              key={it.tool_call_id || i}
                              style={{
                                borderBottom: '1px solid #f0f0f0',
                                padding: '8px 0',
                              }}
                            >
                              <div>
                                <Tag
                                  color={RISK_COLOR[it.risk] || 'red'}
                                >
                                  {it.risk}
                                </Tag>
                                <strong>{it.title || it.name}</strong>
                                <span
                                  style={{
                                    color: '#999',
                                    fontSize: 12,
                                    marginLeft: 6,
                                  }}
                                >
                                  {it.name}
                                </span>
                              </div>
                              {pathNote || it.reason ? (
                                <p
                                  style={{
                                    color: sensitive
                                      ? '#cf1322'
                                      : '#d46b08',
                                    fontSize: 12,
                                    margin: '4px 0',
                                  }}
                                >
                                  {pathNote}
                                  {it.reason
                                    ? `${pathNote ? ' ' : ''}${it.reason}`
                                    : ''}
                                </p>
                              ) : null}
                              <details>
                                <summary
                                  style={{
                                    cursor: 'pointer',
                                    fontSize: 12,
                                    color: '#666',
                                  }}
                                >
                                  {intl.formatMessage({
                                    id: 'ai.panel.parameters',
                                  })}
                                </summary>
                                <pre style={APPROVAL_ARGS_PRE_STYLE}>
                                  {argsText}
                                </pre>
                              </details>
                            </div>
                          );
                        })}
                        <p style={{ color: '#999', fontSize: 12 }}>
                          {intl.formatMessage({ id: 'ai.approval.batch-hint' })}
                          {allowFullControl
                            ? intl.formatMessage({
                                id: 'ai.approval.full-control-hint',
                              })
                            : ''}
                        </p>
                      </div>
                    ),
                    okText: intl.formatMessage({ id: 'ai.approve.once' }),
                    cancelText: intl.formatMessage({ id: 'ai.approve.reject' }),
                    onOk: () => batchResult('allow'),
                    onCancel: () => batchResult('deny'),
                    // 命令式 Modal 挂在自建的 React root 上，拿不到 IntlProvider，
                    // 因此只能用 useIntl() 的 intl 预先求值，不能用 <FormattedMessage/>。
                    footer: (
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'flex-end',
                          gap: 8,
                        }}
                      >
                        <Button
                          danger
                          onClick={() => {
                            batchModal.destroy();
                            batchResult('deny');
                          }}
                        >
                          {intl.formatMessage({ id: 'ai.approve.all-reject' })}
                        </Button>
                        {allowFullControl ? (
                          <Button
                            onClick={() => {
                              batchModal.destroy();
                              batchResult('full_control');
                            }}
                          >
                            {intl.formatMessage({
                              id: 'ai.approve.fully-control',
                            })}
                          </Button>
                        ) : null}
                        <Button
                          onClick={() => {
                            batchModal.destroy();
                            batchResult('once_allow');
                          }}
                        >
                          {intl.formatMessage({
                            id: 'ai.approve.session-allow',
                          })}
                        </Button>
                        <Button
                          type="primary"
                          onClick={() => {
                            batchModal.destroy();
                            batchResult('allow');
                          }}
                        >
                          {intl.formatMessage({ id: 'ai.approve.all-allow' })}
                        </Button>
                      </div>
                    ),
                  });

                  continue;
                }

                if (eventType === 'tool_confirm') {
                  console.log('tool_confirm', parsed);
                  // P0: 主会话写操作审批 — 用 antd Modal.confirm 命令式 API
                  // 绕过 React 渲染队列，Modal 立即弹出，不受 setMessages 积压影响
                  const toolCallId = parsed.tool_call_id || '';
                  const toolName = parsed.name || '';
                  const toolArgs =
                    typeof parsed.arguments === 'object'
                      ? JSON.stringify(parsed.arguments, null, 2)
                      : parsed.arguments || '';
                  // P0-2: 路径分类 + 完全控制可用性（由后端 tool_confirm 事件下发）
                  const allowFullControl = parsed.allow_full_control === true;
                  const pathClass = parsed.path_class || '';
                  const pathReason = parsed.reason || '';

                  const approvalResult = async (
                    decision:
                      | 'allow'
                      | 'once_allow'
                      | 'deny'
                      | 'full_control',
                  ) => {
                    await anqiAiToolConfirm({
                      tool_call_id: toolCallId,
                      decision,
                    });
                  };

                  const modal = Modal.confirm({
                    title: intl.formatMessage(
                      { id: 'ai.approval.write-title' },
                      { name: toolName },
                    ),
                    content: (
                      <div>
                        <pre
                          style={{
                            background: '#f5f5f5',
                            padding: 12,
                            borderRadius: 4,
                            maxHeight: 300,
                            overflow: 'auto',
                            fontSize: 12,
                            whiteSpace: 'pre-wrap',
                            wordBreak: 'break-all',
                            margin: '8px 0',
                          }}
                        >
                          {toolArgs}
                        </pre>
                        {pathClass ? (
                          <p
                            style={{
                              color:
                                pathClass === 'sensitive'
                                  ? '#cf1322'
                                  : '#d46b08',
                              fontSize: 12,
                              margin: '4px 0',
                            }}
                          >
                            {intl.formatMessage(
                              { id: 'ai.approval.path-class' },
                              { class: pathClass },
                            )}
                            {pathReason
                              ? intl.formatMessage(
                                  { id: 'ai.approval.path-reason' },
                                  { reason: pathReason },
                                )
                              : ''}
                          </p>
                        ) : null}
                        <p style={{ color: '#999', fontSize: 12 }}>
                          {intl.formatMessage({
                            id: 'ai.approval.single-hint',
                          })}
                          {allowFullControl
                            ? intl.formatMessage({
                                id: 'ai.approval.full-control-hint',
                              })
                            : ''}
                        </p>
                      </div>
                    ),
                    okText: intl.formatMessage({ id: 'ai.approve.once' }),
                    cancelText: intl.formatMessage({ id: 'ai.approve.reject' }),
                    onOk: () => approvalResult('allow'),
                    onCancel: () => approvalResult('deny'),
                    footer: (
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'flex-end',
                          gap: 8,
                        }}
                      >
                        <Button
                          danger
                          onClick={() => {
                            modal.destroy();
                            approvalResult('deny');
                          }}
                        >
                          {intl.formatMessage({ id: 'ai.approve.reject' })}
                        </Button>
                        {allowFullControl ? (
                          <Button
                            onClick={() => {
                              modal.destroy();
                              approvalResult('full_control');
                            }}
                          >
                            {intl.formatMessage({
                              id: 'ai.approve.fully-control',
                            })}
                          </Button>
                        ) : null}
                        <Button
                          onClick={() => {
                            modal.destroy();
                            approvalResult('once_allow');
                          }}
                        >
                          {intl.formatMessage({
                            id: 'ai.approve.session-allow-tool',
                          })}
                        </Button>
                        <Button
                          type="primary"
                          onClick={() => {
                            modal.destroy();
                            approvalResult('allow');
                          }}
                        >
                          {intl.formatMessage({ id: 'ai.approve.once' })}
                        </Button>
                      </div>
                    ),
                  });

                  continue;
                }

                if (eventType === 'tool_result') {
                  const toolCallId = parsed.tool_call_id;
                  if (toolCallId) {
                    for (let i = segments.length - 1; i >= 0; i--) {
                      const seg = segments[i];
                      if (
                        seg.type === 'tool_call' &&
                        seg.toolCallId === toolCallId
                      ) {
                        seg.status = 'completed';
                        seg.result =
                          typeof parsed.result === 'object'
                            ? JSON.stringify(parsed.result)
                            : parsed.result || '';
                        break;
                      }
                    }
                  }
                  flushSegments();
                  continue;
                }

                if (eventType === 'warning' && (parsed.message || parsed.v)) {
                  segments.push({
                    type: 'warning',
                    content: parsed.message || parsed.v || '',
                  });
                  flushSegments();
                  continue;
                }

                if (eventType === 'iframe-reload') {
                  // 前端 AI 编辑器：AI 写模板成功后刷新 iframe 预览
                  onIframeReload?.();
                  continue;
                }

                if (eventType === 'error') {
                  const errMsg = parsed.message?.content || 'An error occurred';
                  segments.push({ type: 'text', content: errMsg });
                  flushSegments();
                  continue;
                }

                if (
                  eventType === 'usage' &&
                  parsed.total_tokens !== undefined
                ) {
                  setTokenUsage((prev) => ({
                    prompt: prev.prompt + (parsed.prompt_tokens || 0),
                    completion:
                      prev.completion + (parsed.completion_tokens || 0),
                    total: prev.total + (parsed.total_tokens || 0),
                  }));
                  continue;
                }
              } catch (e) {
                console.error('Failed to parse SSE data:', e);
              }
            }
          }
        } finally {
          reader.releaseLock();
          setLoading(false);
        }
      };

      await readStream();
    } catch (error: any) {
      console.error('Chat error:', error);
      const sendFailed = intl.formatMessage({ id: 'ai.chat.send-failed' });
      message.error(error.message || sendFailed);
      setErrorMsg(error.message || sendFailed);
      setLoading(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const loadAgentList = async () => {
    setAgentListLoading(true);
    try {
      const res = await getAiAgents();
      if (Array.isArray(res.data)) setAgentList(res.data);
    } catch (_) {}
    setAgentListLoading(false);
  };

  const handleOpenAgentDrawer = () => {
    setAgentDrawerVisible(true);
    loadAgentList();
  };

  const handleDeleteAgent = async (agent: any) => {
    //message.loading('删除中...', 0);
    handleSend(
      intl.formatMessage(
        { id: 'ai.prompt.agent-delete' },
        { id: agent.id },
      ),
    );
    setAgentDrawerVisible(false);
  };

  const handleToggleAgent = async (agent: any, enabled: number) => {
    handleSend(
      intl.formatMessage(
        { id: 'ai.prompt.agent-toggle' },
        {
          id: agent.id,
          state: intl.formatMessage({
            id: enabled === 1 ? 'ai.panel.agent-enable' : 'ai.panel.agent-pause',
          }),
        },
      ),
    );
    setAgentDrawerVisible(false);
  };

  const handleRunAgent = async (agent: any) => {
    handleSend(
      intl.formatMessage({ id: 'ai.prompt.agent-run' }, { id: agent.id }),
    );
    setAgentDrawerVisible(false);
  };

  const handleChatAgent = async (agent: any) => {
    handleSend(
      intl.formatMessage({ id: 'ai.prompt.agent-chat' }, { id: agent.id }),
    );
    setAgentDrawerVisible(false);
  };

  // ── Skill 管理 ──

  const loadSkillList = async () => {
    setSkillListLoading(true);
    try {
      const res = await getSkillList();
      if (Array.isArray(res.data)) setSkillList(res.data);
    } catch (_) {}
    setSkillListLoading(false);
  };

  const handleOpenSkillsList = () => {
    setSkillDrawerVisible(true);
    loadSkillList();
  };

  const handleDeleteSkill = async (skill: any) => {
    //message.loading('删除中...', 0);
    try {
      const res = await anqiSkillDelete({ name: skill.name });
      //message.destroy();
      if (res.code === 0) {
        message.success(intl.formatMessage({ id: 'ai.chat.delete-success' }));
        loadSkillList();
      } else {
        message.error(
          res.msg || intl.formatMessage({ id: 'ai.chat.delete-failed' }),
        );
      }
    } catch (_) {
      //message.destroy();
      message.error(intl.formatMessage({ id: 'ai.chat.delete-failed' }));
    }
  };

  const handleReloadSkills = async () => {
    message.loading(intl.formatMessage({ id: 'ai.chat.reloading' }), 0);
    try {
      const res = await anqiSkillReload();
      message.destroy();
      if (res.code === 0) {
        message.success(intl.formatMessage({ id: 'ai.chat.reload-success' }));
        loadSkillList();
      } else {
        message.error(
          res.msg || intl.formatMessage({ id: 'ai.chat.reload-failed' }),
        );
      }
    } catch (_) {
      message.destroy();
      message.error(intl.formatMessage({ id: 'ai.chat.reload-failed' }));
    }
  };

  const handleViewSkill = async (skill: any) => {
    const none = intl.formatMessage({ id: 'ai.panel.none' });
    handleSend(
      intl.formatMessage(
        { id: 'ai.prompt.skill-view' },
        {
          name: skill.name,
          description: skill.description || none,
          category: skill.category || none,
          version: skill.version || none,
          tags: skill.tags?.join(', ') || none,
          fileCount: skill.file_count,
          updatedAt: skill.updated_at || none,
        },
      ),
    );
    setSkillDrawerVisible(false);
  };

  const handleModelChange = (model: string) => {
    setSelectedModel(model);
    setStore('aiChatModel', model);
  };

  const handleUploadFile = (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    const hide = message.loading(
      intl.formatMessage({ id: 'ai.chat.uploading' }),
      0,
    );
    anqiAiUpload(formData)
      .then((res: any) => {
        if (res.code === 0) {
          setUploadedFiles((prev) => [
            ...prev,
            {
              file_name: res.data.file_name,
              file_path: res.data.file_path,
              file_type: 'attachment',
            },
          ]);
          message.info(intl.formatMessage({ id: 'ai.chat.upload-success' }));
        } else {
          message.info(
            res.msg || intl.formatMessage({ id: 'ai.chat.upload-failed' }),
          );
        }
      })
      .finally(() => hide());
    return false;
  };

  const handleRemoveFile = (index: number) => {
    setUploadedFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handlePaste = (e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items;
    if (!items) return;
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      if (item.type.startsWith('image/')) {
        e.preventDefault();
        const file = item.getAsFile();
        if (file) {
          handleUploadFile(file);
        }
        break;
      }
    }
  };

  const handleDropFile = (e: React.DragEvent) => {
    e.preventDefault();
    const files = e.dataTransfer?.files;
    if (files && files.length > 0) {
      for (let i = 0; i < files.length; i++) {
        handleUploadFile(files[i]);
      }
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
  };

  const handleSaveCustomProvider = (values: any) => {
    if (!values.name || !values.base_url || !values.api_key || !values.model) {
      message.warning(
        intl.formatMessage({ id: 'ai.settings.fill-required' }),
      );
      return;
    }
    const provider = Object.assign({}, editProvider, values);
    if (editProviderIndex === -1) {
      customProviders.push(provider);
    } else {
      customProviders[editProviderIndex] = provider;
    }

    saveAiSettings(customProviders)
      .then((res: any) => {
        if (res.code === 0) {
          if (Array.isArray(res.data)) setCustomProviders(res.data);
          setEditProviderModalVisible(false);
          setEditProvider(null);
          message.success(intl.formatMessage({ id: 'ai.chat.save-success' }));
        } else {
          message.info(
            res.msg || intl.formatMessage({ id: 'ai.chat.save-failed' }),
          );
        }
      })
      .catch(() => message.error(intl.formatMessage({ id: 'ai.chat.save-failed' })));
  };

  const handleDeleteCustomProvider = (index: number) => {
    customProviders.splice(index, 1);
    saveAiSettings(customProviders)
      .then((res: any) => {
        if (res.code === 0) {
          if (Array.isArray(res.data)) {
            setCustomProviders(res.data);
          }
          setEditProvider(null);
          message.success(intl.formatMessage({ id: 'ai.chat.delete-success' }));
        } else {
          message.info(
            res.msg || intl.formatMessage({ id: 'ai.chat.delete-failed' }),
          );
        }
      })
      .catch(
        () => message.error(intl.formatMessage({ id: 'ai.chat.delete-success' })),
      );
  };

  const handleOpenEditProvider = (
    index: number,
    provider?: AiProviderConfig,
  ) => {
    setEditProviderIndex(index);
    setEditProvider(
      provider || {
        name: '',
        base_url: '',
        api_key: '',
        model: '',
        enable_reasoning: true,
        max_tokens: 8192,
      },
    );
    setEditProviderModalVisible(true);
  };

  const handleOpenTemplateSelect = () => {
    setTemplateLoading(true);
    setTemplateModalVisible(true);
    getDesignInfo({})
      .then((res: any) => {
        let data = res.data || {};
        let tmpList = []
          .concat(
            (data.tpl_files || []).map((item: any) => {
              item.name = item.path;
              item.path = 'template/' + data.package + '/' + item.path;
              return item;
            }),
          )
          .concat(
            (data.static_files || []).map((item: any) => {
              item.name = item.path;
              item.path = 'public/static/' + data.package + '/' + item.path;
              return item;
            }),
          );
        setTemplateFiles(tmpList);
      })
      .catch(
        () =>
          message.error(
            intl.formatMessage({ id: 'ai.chat.template-load-failed' }),
          ),
      )
      .finally(() => setTemplateLoading(false));
  };

  const handleSelectTemplateFile = (file: any) => {
    setUploadedFiles((prev) => [
      ...prev,
      {
        file_name: file.remark || file.path.split('/').pop(),
        file_path: file.path,
        file_type: 'template',
      },
    ]);
    setTemplateModalVisible(false);
  };

  const selectItems: MenuProps['items'] = [
    {
      key: 'upload',
      label: (
        <Upload
          accept="*"
          showUploadList={false}
          customRequest={(e) => {
            handleUploadFile(e.file as File);
          }}
        >
          <PaperClipOutlined />{' '}
          <FormattedMessage id="ai.chat.upload-file" />
        </Upload>
      ),
    },
    {
      key: 'select',
      label: (
        <div onClick={handleOpenTemplateSelect}>
          <CloudServerOutlined />{' '}
          <FormattedMessage id="ai.chat.select-template" />
        </div>
      ),
    },
  ];

  if (!visible) return null;

  // =====================================================================
  // JSX
  // =====================================================================
  return (
    <div className={`ai-chat-modal${maximized ? ' maximized' : ''}`}>
      <Header
        maximized={maximized}
        onClose={onClose}
        onToggleMaximize={() => setMaximized(!maximized)}
        onNewSession={handleNewSession}
        onOpenAgentDrawer={handleOpenAgentDrawer}
        onOpenSettings={handleOpenSettings}
        onOpenSkillsList={handleOpenSkillsList}
        onOpenTrace={handleOpenTraceDrawer}
        sessionListOpen={sessionListOpen}
        sessionListLoading={sessionListLoading}
        sessionList={sessionList}
        onOpenSessionList={handleLoadSessions}
        onSwitchSession={handleSwitchSession}
        onCloseSessionList={() => setSessionListOpen(false)}
      />

      <ChatMessages
        messages={messages}
        loading={loading}
        onSend={handleSend}
        historyLoading={historyLoading}
        errorMsg={errorMsg}
        messagesEndRef={messagesEndRef}
      />

      {(iframeUrl || selectedDom) && (
        <div
          style={{
            padding: '4px 12px',
            fontSize: 12,
            color: '#666',
            background: '#fafafa',
            borderBottom: '1px solid #f0f0f0',
            display: 'flex',
            gap: 8,
            alignItems: 'center',
            flexWrap: 'wrap',
          }}
        >
          {iframeUrl && (
            <span
              style={{
                maxWidth: 240,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
              title={iframeUrl}
            >
              🔗 {iframeUrl}
            </span>
          )}
          {selectedDom && (
            <span
              style={{
                color: '#d46b08',
                maxWidth: 280,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
              title={selectedDom}
            >
              🎯 {selectedDom.slice(0, 50)}
              {selectedDom.length > 50 ? '...' : ''}
            </span>
          )}
        </div>
      )}

      <ChatInput
        inputValue={inputValue}
        loading={loading}
        uploadedFiles={uploadedFiles}
        selectedModel={selectedModel}
        selectItems={selectItems}
        tokenUsage={tokenUsage}
        customProviders={customProviders}
        onInputChange={setInputValue}
        onKeyPress={handleKeyPress}
        onPaste={handlePaste}
        onSend={() => handleSend()}
        onModelChange={handleModelChange}
        onRemoveFile={handleRemoveFile}
        onDropFile={handleDropFile}
        onDragOver={handleDragOver}
      />

      <TemplateModal
        visible={templateModalVisible}
        loading={templateLoading}
        files={templateFiles}
        onClose={() => setTemplateModalVisible(false)}
        onSelect={handleSelectTemplateFile}
      />

      <SettingsPanel
        visible={settingsModalVisible}
        onClose={() => setSettingsModalVisible(false)}
        customProviders={customProviders}
        onAdd={() => handleOpenEditProvider(-1)}
        onEdit={(index, provider) => handleOpenEditProvider(index, provider)}
        onDelete={handleDeleteCustomProvider}
      />

      {editProviderModalVisible && (
        <ProviderFormModal
          visible={editProviderModalVisible}
          editProvider={editProvider}
          editIndex={editProviderIndex}
          onClose={() => {
            setEditProviderModalVisible(false);
            setEditProvider(null);
            setEditProviderIndex(-1);
          }}
          onSave={handleSaveCustomProvider}
        />
      )}

      <AgentDrawer
        visible={agentDrawerVisible}
        loading={agentListLoading}
        agents={agentList}
        onClose={() => setAgentDrawerVisible(false)}
        onRun={handleRunAgent}
        onChat={handleChatAgent}
        onLogs={handleShowAgentLogs}
        onToggle={handleToggleAgent}
        onDelete={handleDeleteAgent}
      />

      <AgentLogDrawer
        visible={agentLogsVisible}
        loading={agentLogsLoading}
        agent={selectedAgent}
        logs={agentLogs}
        onClose={() => setAgentLogsVisible(false)}
      />

      <SkillListDrawer
        visible={skillDrawerVisible}
        loading={skillListLoading}
        skills={skillList}
        onClose={() => setSkillDrawerVisible(false)}
        onView={handleViewSkill}
        onReload={handleReloadSkills}
        onDelete={handleDeleteSkill}
      />

      <Drawer
        title={intl.formatMessage({ id: 'ai.panel.trace' })}
        width={520}
        open={traceDrawerVisible}
        onClose={() => setTraceDrawerVisible(false)}
      >
        {traceLoading ? (
          <div style={{ textAlign: 'center', padding: '32px 0' }}>
            <Spin />
          </div>
        ) : traceEvents.length === 0 ? (
          <Empty
            description={intl.formatMessage({ id: 'ai.panel.no-trace-events' })}
          />
        ) : (
          <div style={{ fontSize: 13 }}>
            {[...traceEvents].reverse().map((ev: any, i: number) => (
              <div
                key={i}
                style={{
                  borderBottom: '1px solid #f0f0f0',
                  padding: '8px 0',
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    flexWrap: 'wrap',
                  }}
                >
                  <Tag color={TRACE_PHASE_COLOR[ev.phase] || 'default'}>
                    {ev.phase}
                  </Tag>
                  <span style={{ color: '#999', fontSize: 12 }}>
                    {ev.ts
                      ? new Date(ev.ts).toLocaleTimeString('zh-CN', {
                          hour12: false,
                        })
                      : ''}
                  </span>
                  {ev.duration_ms ? (
                    <span style={{ color: '#999', fontSize: 12 }}>
                      {ev.duration_ms}ms
                    </span>
                  ) : null}
                  {ev.err ? <Tag color="red">error</Tag> : null}
                  {ev.decision ? (
                    <Tag
                      color={TRACE_DECISION_COLOR[ev.decision] || 'default'}
                    >
                      {ev.decision}
                    </Tag>
                  ) : null}
                </div>
                <div style={{ marginTop: 4, fontWeight: 500 }}>
                  {ev.name}
                </div>
                {ev.detail ? (
                  <div
                    style={{
                      marginTop: 2,
                      color: '#666',
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-all',
                    }}
                  >
                    {ev.detail}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        )}
      </Drawer>
    </div>
  );
};

export default AiChat;
