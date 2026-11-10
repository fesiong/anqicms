import { FormattedMessage } from '@umijs/max';
import { useState } from 'react';

/** 思维链条折叠展示 */
const ThinkingBlock: React.FC<{ content: string }> = ({ content }) => {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="ai-chat-thinking">
      <div
        className="ai-chat-thinking-header"
        onClick={() => setExpanded(!expanded)}
      >
        <span
          className={`ai-chat-thinking-arrow ${expanded ? 'expanded' : ''}`}
        >
          ▶
        </span>
        <span className="ai-chat-thinking-label">
          <FormattedMessage id="ai.thinking" />
        </span>
      </div>
      {expanded && (
        <div className="ai-chat-thinking-body">
          <pre>{content}</pre>
        </div>
      )}
    </div>
  );
};

export default ThinkingBlock;
