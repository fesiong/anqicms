import config from '@/services/config';
import { getSessionStore, getStore } from '@/utils/store';
import {
  AlignLeftOutlined,
  AuditOutlined,
  CloseOutlined,
  EditOutlined,
  GlobalOutlined,
  ShareAltOutlined,
  StarOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import gfm from '@bytemd/plugin-gfm';
import { Viewer } from '@bytemd/react';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Input, message, Spin } from 'antd';
import { getProcessor } from 'bytemd';
import 'bytemd/dist/index.css';
import { useRef, useState } from 'react';
import './index.less';

const plugins = [gfm()];
const processor = getProcessor({ plugins });
// AI Editor Overlay Component
const AiEditorOverlay: React.FC<{
  visible: boolean;
  onClose: () => void;
  getSelection: () => string;
  appendBlock: (text: string, htmlText: string) => void;
  replaceSelection: (text: string, htmlText: string) => void;
  setContent: (text: string, htmlText: string) => void;
  getContent: () => string;
}> = ({
  visible,
  onClose,
  getSelection,
  appendBlock,
  replaceSelection,
  setContent,
  getContent,
}) => {
  const [inputValue, setInputValue] = useState('');
  const [selectedTag, setSelectedTag] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState('');
  const [finished, setFinished] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const inputRef = useRef<any>(null);
  const intl = useIntl();

  if (!visible) return null;

  const resetState = () => {
    setInputValue('');
    setSelectedTag('');
    setLoading(false);
    setResult('');
    setFinished(false);
    abortRef.current?.abort();
    abortRef.current = null;
  };

  const handleClose = () => {
    resetState();
    onClose();
  };

  const handleSubmit = async (prompt: string, tag: string) => {
    if (!prompt.trim() && !tag) return;

    setLoading(true);
    setResult('');
    setFinished(false);

    const adminToken = getSessionStore('adminToken') || getStore('adminToken');
    const selection = getSelection();
    const content = getContent();
    const body: any = {
      prompt: prompt || tag,
      instructions: selection
        ? `Selected text:\n${selection}`
        : `Document:\n${content}`,
    };

    const abortController = new AbortController();
    abortRef.current = abortController;

    try {
      const response = await fetch(config.baseUrl + '/anqi/ai/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Admin: adminToken,
        },
        body: JSON.stringify(body),
        signal: abortController.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

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

          if (eventType === 'end') {
            setFinished(true);
            setLoading(false);
            return;
          }

          if (eventType === 'error') {
            throw new Error(
              data ||
                intl.formatMessage({ id: 'component.aiEditor.error.request-error' }),
            );
          }

          if (data) {
            try {
              const parsed = JSON.parse(data);
              tmpContent += parsed.v || '';
              setResult(tmpContent);
            } catch {}
            setResult(tmpContent);
          }
        }
      }

      setFinished(true);
    } catch (error: any) {
      if (error.name === 'AbortError') return;
      message.error(
        error.message ||
          intl.formatMessage({ id: 'component.aiEditor.error.request-failed' }),
      );
    } finally {
      setLoading(false);
    }
  };

  const quickTags = [
    {
      id: 'improve',
      label: intl.formatMessage({ id: 'ai.action.improve-writing' }),
      prompt:
        'Improve the writing while preserving its meaning and tone. Return only the revised text.',
      icon: <ThunderboltOutlined />,
    },
    {
      id: 'proofread',
      label: intl.formatMessage({ id: 'ai.action.fix-spelling-and-grammar' }),
      prompt:
        'Correct spelling, grammar, punctuation, and usage. Return only the corrected text.',
      icon: <AuditOutlined />,
    },
    {
      id: 'simplify',
      label: intl.formatMessage({ id: 'ai.action.simplify' }),
      prompt:
        'Make this clearer and easier to understand without losing important information. Return only the revised text.',
      icon: <EditOutlined />,
    },
    {
      id: 'expand',
      label: intl.formatMessage({ id: 'ai.action.expand' }),
      prompt:
        'Expand this with useful detail while preserving the original meaning and tone. Return only the revised text.',
      icon: <StarOutlined />,
    },
    {
      id: 'format',
      label: intl.formatMessage({ id: 'ai.action.format' }),
      prompt:
        'Format this as clean, well-structured markdown. Return only the formatted text.',
      icon: <AlignLeftOutlined />,
    },
    {
      id: 'translate',
      label: intl.formatMessage({ id: 'ai.action.translate' }),
      prompt:
        'Translate this into English. If it is already English, translate it into Simplified Chinese. Return only the translation.',
      icon: <GlobalOutlined />,
    },
    {
      id: 'summarize',
      label: intl.formatMessage({ id: 'ai.action.summarize' }),
      prompt: 'Summarize this concisely. Return only the summary.',
      icon: <ShareAltOutlined />,
    },
  ];

  const handleAppend = async () => {
    const htmlText = String(await processor.process(result));
    appendBlock(result, htmlText);
    handleClose();
  };

  const handleReplace = async () => {
    const htmlText = String(await processor.process(result));
    const selection = getSelection();
    if (selection) {
      replaceSelection(result, htmlText);
    } else {
      setContent(result, htmlText);
    }
    handleClose();
  };

  const handleDiscard = () => {
    handleClose();
  };

  return (
    <div className="ai-editor-overlay">
      <div className="ai-editor-panel">
        <div className="ai-editor-header">
          <span>
            <FormattedMessage id="component.aiEditor.title" />
          </span>
          <CloseOutlined className="ai-editor-close" onClick={handleClose} />
        </div>
        <div className="ai-editor-body">
          <Input.TextArea
            ref={inputRef}
            className="ai-editor-input"
            placeholder={intl.formatMessage({
              id: 'component.aiEditor.input-placeholder',
            })}
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onPressEnter={(e) => {
              if (!e.shiftKey) {
                e.preventDefault();
                handleSubmit(inputValue, selectedTag);
              }
            }}
            autoSize={{ minRows: 2, maxRows: 4 }}
            disabled={loading}
          />

          {!loading && !finished && (
            <div className="ai-editor-tags">
              <div className="ai-editor-tags-hint">
                <ThunderboltOutlined />{' '}
                <FormattedMessage id="component.aiEditor.quick-actions" />
              </div>
              <div className="ai-editor-tags-list">
                {quickTags.map((tag) => (
                  <div
                    key={tag.id}
                    className={`ai-editor-tag ${
                      selectedTag === tag.id ? 'active' : ''
                    }`}
                    onClick={() => {
                      setSelectedTag(tag.id);
                      handleSubmit(tag.prompt, tag.id);
                    }}
                  >
                    <span className="ai-editor-tag-icon">{tag.icon}</span>
                    {tag.label}
                  </div>
                ))}
              </div>
            </div>
          )}

          {loading && (
            <div className="ai-editor-loading">
              <Spin size="small" />
              <span>
                <FormattedMessage id="component.aiEditor.processing" />
              </span>
            </div>
          )}

          {finished && result && (
            <div className="ai-editor-result-section">
              <div className="ai-editor-result-label">
                <FormattedMessage id="component.aiEditor.output-label" />
              </div>
              <div className="ai-editor-result-content">
                <Viewer value={result} plugins={plugins} />
              </div>
              <div className="ai-editor-result-check">
                <FormattedMessage id="component.aiEditor.check-result" />
              </div>
              <div className="ai-editor-result-actions">
                <Button size="small" type="primary" onClick={handleAppend}>
                  <FormattedMessage id="component.aiEditor.append" />
                </Button>
                <Button size="small" onClick={handleReplace}>
                  <FormattedMessage id="component.aiEditor.replace" />
                </Button>
                <Button size="small" danger onClick={handleDiscard}>
                  <FormattedMessage id="component.aiEditor.discard" />
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default AiEditorOverlay;
