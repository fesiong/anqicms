/** 系统警告 */
const WarningBlock: React.FC<{ message: string }> = ({ message: msg }) => (
  <div className="ai-chat-warning">
    <span className="ai-chat-warning-icon">⚠️</span>
    <span className="ai-chat-warning-text">{msg}</span>
  </div>
);

export default WarningBlock;
