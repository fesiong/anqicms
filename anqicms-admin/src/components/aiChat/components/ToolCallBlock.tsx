import { FormattedMessage } from '@umijs/max';
import { useState } from 'react';

/** 单个工具调用块 */
const ToolCallBlock: React.FC<{
  toolName: string;
  arguments: string;
  status: 'calling' | 'completed' | 'denied';
  result?: string;
}> = ({ toolName: name, arguments: args, status: st, result: res }) => {
  const [expanded, setExpanded] = useState(false);
  return (
    <div
      className="ai-chat-toolcall"
      style={{
        margin: '4px 0',
        padding: '6px 8px',
        background: '#f5f5f5',
        borderRadius: 6,
        fontSize: 13,
        lineHeight: 1.5,
      }}
    >
      <div
        className="ai-chat-toolcall-header"
        onClick={() => setExpanded(!expanded)}
        style={{ cursor: 'pointer', userSelect: 'none' }}
      >
        <span
          className={`ai-chat-thinking-arrow ${expanded ? 'expanded' : ''}`}
        >
          ▶
        </span>
        {st === 'denied' ? '🚫' : st === 'calling' ? '🔄' : '✅'}{' '}
        <strong>{name}</strong>
        {st === 'calling' ? (
          <>
            {' '}
            <FormattedMessage id="ai.toolcall.running" />
          </>
        ) : st === 'denied' ? (
          <>
            {' '}
            <FormattedMessage id="ai.toolcall.denied" />
          </>
        ) : (
          ''
        )}
      </div>
      {expanded && (
        <div className="ai-chat-toolcall-body" style={{ marginTop: 4 }}>
          <pre
            style={{
              fontSize: 12,
              background: '#fff',
              padding: 4,
              borderRadius: 4,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
              margin: 0,
            }}
          >
            {args.length > 200 ? args.slice(0, 200) + '...' : args}
          </pre>
          {res && (
            <pre
              style={{
                fontSize: 12,
                background: '#f6ffed',
                padding: 4,
                borderRadius: 4,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
                margin: '4px 0 0 0',
              }}
            >
              {res.length > 300 ? res.slice(0, 300) + '...' : res}
            </pre>
          )}
        </div>
      )}
    </div>
  );
};

export default ToolCallBlock;
