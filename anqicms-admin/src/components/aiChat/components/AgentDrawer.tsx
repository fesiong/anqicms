import { useIntl } from '@umijs/max';
import { Button, Drawer, List, Spin } from 'antd';

interface AgentDrawerProps {
  visible: boolean;
  loading: boolean;
  agents: any[];
  onClose: () => void;
  onRun: (agent: any) => void;
  onChat: (agent: any) => void;
  onLogs: (agent: any) => void;
  onToggle: (agent: any, enabled: number) => void;
  onDelete: (agent: any) => void;
}

const AgentDrawer: React.FC<AgentDrawerProps> = ({
  visible,
  loading,
  agents,
  onClose,
  onRun,
  onChat,
  onLogs,
  onToggle,
  onDelete,
}) => {
  const intl = useIntl();
  return (
    <Drawer
      title={intl.formatMessage({ id: 'ai.panel.agent-title' })}
      placement="right"
      className="agent-drawer"
      open={visible}
      onClose={onClose}
      width={520}
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: '40px 0' }}>
          <Spin size="small" />
        </div>
      ) : agents.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
          {intl.formatMessage({ id: 'ai.panel.no-agents' })}
          <br />
          {intl.formatMessage({ id: 'ai.panel.create-hint' })}
          <br />
          <em>
            {intl.formatMessage({ id: 'ai.panel.agent-create-example' })}
          </em>
        </div>
      ) : (
        <List
          size="small"
          dataSource={agents}
          itemLayout="vertical"
          renderItem={(item: any) => (
            <List.Item
              actions={[
                <Button
                  key="run"
                  type="link"
                  size="small"
                  onClick={() => onRun(item)}
                >
                  {intl.formatMessage({ id: 'ai.panel.agent-run' })}
                </Button>,
                <Button
                  key="chat"
                  type="link"
                  size="small"
                  onClick={() => onChat(item)}
                >
                  {intl.formatMessage({ id: 'ai.panel.agent-chat' })}
                </Button>,
                <Button
                  key="logs"
                  type="link"
                  size="small"
                  onClick={() => onLogs(item)}
                >
                  {intl.formatMessage({ id: 'ai.panel.agent-logs' })}
                </Button>,
                <Button
                  key="toggle"
                  type="link"
                  size="small"
                  onClick={() => onToggle(item, item.enabled === 1 ? 0 : 1)}
                >
                  {item.enabled === 1
                    ? intl.formatMessage({ id: 'ai.panel.agent-pause' })
                    : intl.formatMessage({ id: 'ai.panel.agent-enable' })}
                </Button>,
                <Button
                  key="delete"
                  type="link"
                  size="small"
                  danger
                  onClick={() => onDelete(item)}
                >
                  {intl.formatMessage({ id: 'ai.panel.agent-delete' })}
                </Button>,
              ]}
            >
              <List.Item.Meta
                title={
                  <div style={{ fontSize: 13 }}>
                    <span
                      style={{
                        display: 'inline-block',
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        background: item.enabled === 1 ? '#52c41a' : '#d9d9d9',
                        marginRight: 6,
                      }}
                    />
                    {item.name ||
                      intl.formatMessage(
                        { id: 'ai.panel.agent-unnamed' },
                        { id: item.id },
                      )}
                  </div>
                }
                description={
                  <div style={{ fontSize: 12, color: '#999' }}>
                    {item.cron_expr
                      ? `⏰ ${item.cron_expr}`
                      : `🔘 ${intl.formatMessage({
                          id: 'ai.panel.manual-only',
                        })}`}
                    {' | '}
                    {intl.formatMessage(
                      { id: 'ai.panel.run-count' },
                      { count: item.run_count || 0 },
                    )}
                    {item.last_run_at > 0
                      ? ` | ${intl.formatMessage(
                          { id: 'ai.panel.last-run' },
                          {
                            time: new Date(
                              item.last_run_at * 1000,
                            ).toLocaleString(),
                          },
                        )}`
                      : ''}
                    {item.last_summary && (
                      <div
                        style={{
                          marginTop: 4,
                          padding: '4px 8px',
                          background: '#f5f5f5',
                          borderRadius: 4,
                          maxHeight: 60,
                          overflow: 'hidden',
                        }}
                      >
                        {item.last_summary.slice(0, 200)}
                      </div>
                    )}
                  </div>
                }
              />
            </List.Item>
          )}
        />
      )}
    </Drawer>
  );
};

export default AgentDrawer;
