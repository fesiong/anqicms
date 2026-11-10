import { getAnqiInfo } from '@/services';
import config from '@/services/config';
import { getSessionStore, getStore } from '@/utils/store';
import {
  ProForm,
  ProFormInstance,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Checkbox, Modal, Space, Tag, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import './index.less';
import { parseTdkJson } from './parseTdk';

export type AiGetTdkProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (values: any) => Promise<void>;
  open: boolean;
  content: string;
};

const AiGetTdk: React.FC<AiGetTdkProps> = (props) => {
  const formRef = useRef<ProFormInstance>();
  const [anqiUser, setAnqiUser] = useState<any>({});
  const [plainText, setPlainText] = useState<string>('');
  const [tdk, setTdk] = useState<any>({});
  const [aiContent, setAiContent] = useState<string>('');
  const [parseOk, setParseOk] = useState<boolean>(false);
  const [aiFinished, setAiFinished] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const intl = useIntl();

  useEffect(() => {
    // 获取AIremain
    getAnqiInfo().then((res) => {
      setAnqiUser(res.data || {});
    });
    setPlainText(props.content.replace(/<[^>]*>/g, ''));
  }, []);

  const finishedGenerate = () => {
    let newTdk = {
      description: tdk.description,
      keywords: tdk.keywords_ok ? tdk.keywords : '',
      title: tdk.title_ok ? tdk.title : '',
    };
    props.onSubmit(newTdk);
  };

  const startGenerate = async () => {
    if (loading) {
      return;
    }
    if (props.content.length < 200) {
      message.error(
        intl.formatMessage({ id: 'component.aitdk.content.required' }),
      );
      return;
    }
    setLoading(true);
    setAiContent('');
    setAiFinished(false);
    setParseOk(false);
    setTdk({});

    const adminToken = getSessionStore('adminToken') || getStore('adminToken');
    const body: any = {
      prompt: intl.formatMessage(
        { id: 'component.aitdk.prompt' },
        { content: plainText },
      ),
    };

    try {
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
          jsonResp.msg || intl.formatMessage({ id: 'component.ai.request-failed' }),
        );
      }

      const reader = response.body?.getReader();
      if (!reader) {
        throw new Error('No readable stream');
      }

      const decoder = new TextDecoder();
      let buffer = '';
      let tmpContent = '';

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

          // end 事件：AI 响应结束
          if (eventType === 'end') {
            setAiFinished(true);
            setLoading(false);

            // 尝试解析 JSON（兼容纯 JSON / ```json 代码块 / 重复 JSON / 前后多余文字）
            const parsed = parseTdkJson(tmpContent);
            if (parsed) {
              setParseOk(true);
              setTdk(parsed);
            } else {
              console.error('Error parsing TDK JSON:', tmpContent);
            }
            continue;
          }

          // message 事件：AI 响应内容
          if (eventType === 'message') {
            try {
              const parsed = JSON.parse(data || '{}');
              if (parsed.v !== undefined) {
                tmpContent += parsed.v;
                setAiContent(tmpContent);
              }
            } catch (e) {
              console.error('Error parsing SSE data:', e);
            }
            continue;
          }

          // warning 事件：显示警告
          if (eventType === 'warning') {
            try {
              const parsed = JSON.parse(data || '{}');
              if (parsed.v) {
                message.warning(parsed.v);
              }
            } catch (e) {
              // ignore
            }
            continue;
          }

          // config 事件：AI 配置错误
          if (eventType === 'config') {
            message.error(
              intl.formatMessage({ id: 'component.ai.config-error' }),
            );
            setLoading(false);
            continue;
          }
        }
      }

      // 如果流结束但没有收到 end 事件，也标记完成
      if (!aiFinished) {
        setAiFinished(true);
        setLoading(false);
        const parsed = parseTdkJson(tmpContent);
        if (parsed) {
          setParseOk(true);
          setTdk(parsed);
        } else {
          console.error('Error parsing TDK JSON:', tmpContent);
        }
      }
    } catch (error: any) {
      console.error('TDK generation error:', error);
      message.error(
        error.message ||
          intl.formatMessage({ id: 'component.aitdk.generate-failed' }),
      );
      setLoading(false);
    }
  };

  return (
    <Modal
      width={800}
      title={<FormattedMessage id="component.aitdk.name" />}
      open={props.open}
      onCancel={() => {
        props.onCancel();
      }}
      footer={null}
    >
      <div className="mb-normal">
        <div className="extra-text">
          <Space>
            <span>
              <FormattedMessage id="component.right-content.integral" />
              {anqiUser.integral}
            </span>
            <span>
              <FormattedMessage id="component.right-content.free-token" />
              {anqiUser.free_token}
            </span>
            <span>
              <FormattedMessage id="component.right-content.total-token" />
              {anqiUser.total_token}
            </span>
            <span>
              <FormattedMessage id="component.right-content.un-pay-token" />
              {anqiUser.un_pay_token}
            </span>
            {anqiUser.is_owe_fee === 1 && (
              <Tag color="red">
                <FormattedMessage id="component.right-content.is-owe-fee" />
              </Tag>
            )}
          </Space>
          <div>
            <FormattedMessage id="component.right-content.total-token.description" />
          </div>
        </div>
      </div>
      <ProForm layout="horizontal" formRef={formRef} submitter={false}>
        {aiContent && ((aiFinished && !parseOk) || !aiFinished) && (
          <ProFormTextArea
            label={intl.formatMessage({ id: 'component.aitdk.ai-result' })}
            fieldProps={{
              value: aiContent,
            }}
            extra={
              !parseOk
                ? intl.formatMessage({ id: 'component.aitdk.parse-fail' })
                : ''
            }
          />
        )}
        {aiFinished && (
          <>
            <ProFormText
              label={intl.formatMessage({ id: 'content.title.name' })}
              name={['tdk', 'title']}
              fieldProps={{
                value: tdk.title,
                onChange: (e) => {
                  setTdk({
                    ...tdk,
                    title: e.target.value,
                  });
                },
                suffix: (
                  <Checkbox
                    checked={tdk.title_ok}
                    onChange={(e) => {
                      setTdk({
                        ...tdk,
                        title_ok: e.target.checked,
                      });
                    }}
                  />
                ),
              }}
            />
            <ProFormText
              label={intl.formatMessage({ id: 'content.keywords.name' })}
              name={['tdk', 'keywords']}
              fieldProps={{
                value: tdk.keywords,
                onChange: (e) => {
                  setTdk({
                    ...tdk,
                    keywords: e.target.value,
                  });
                },
                suffix: (
                  <Checkbox
                    checked={tdk.keywords_ok}
                    onChange={(e) => {
                      setTdk({
                        ...tdk,
                        keywords_ok: e.target.checked,
                      });
                    }}
                  />
                ),
              }}
            />
            <ProFormTextArea
              label={intl.formatMessage({ id: 'content.description.name' })}
              name={['tdk', 'description']}
              fieldProps={{
                value: tdk.description,
                onChange: (e) => {
                  setTdk({
                    ...tdk,
                    description: e.target.value,
                  });
                },
              }}
            />
          </>
        )}
        <div className="generate-btn mb-normal">
          <Space size={20}>
            <Button
              onClick={startGenerate}
              loading={loading}
              disabled={aiFinished}
            >
              <FormattedMessage id="component.aitdk.btn.generate" />
            </Button>
            {aiFinished && (
              <>
                <Button onClick={finishedGenerate} loading={loading}>
                  <FormattedMessage id="component.aigenerate.btn.finish" />
                </Button>
                <Button onClick={() => props.onCancel()} loading={loading}>
                  <FormattedMessage id="component.aigenerate.btn.abundant" />
                </Button>
              </>
            )}
          </Space>
        </div>
      </ProForm>
    </Modal>
  );
};

export default AiGetTdk;
