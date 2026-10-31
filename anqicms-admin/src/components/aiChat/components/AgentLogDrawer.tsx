import { useIntl } from '@umijs/max';
import { Viewer } from '@bytemd/react';
import { Button, Drawer, List, Spin, Tag } from 'antd';
import React from 'react';

interface AgentLogDrawerProps {
  visible: boolean;
  loading: boolean;
  agent: any;
  logs: any[];
  onClose: () => void;
}

const statusColor: Record<number, string> = {
  1: 'success',
  2: 'error',
};

const statusKey: Record<number, string> = {
  1: 'ai.panel.log-success',
  2: 'ai.panel.log-failed',
};

const AgentLogDrawer: React.FC<AgentLogDrawerProps> = ({
  visible,
  loading,
  agent,
  logs,
  onClose,
}) => {
  const [openidx, setOpenidx] = React.useState<number | null>(null);
  const intl = useIntl();

  return (
    <Drawer
      title={
        agent
          ? intl.formatMessage(
              { id: 'ai.panel.log-title' },
              { name: agent.name || `#${agent.id}` },
            )
          : intl.formatMessage({ id: 'ai.panel.log' })
      }
      open={visible}
      onClose={onClose}
      width={560}
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: '40px 0' }}>
          <Spin size="small" />
        </div>
      ) : logs.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
          {intl.formatMessage({ id: 'ai.panel.no-logs' })}
        </div>
      ) : (
        <List
          size="small"
          dataSource={logs}
          renderItem={(item: any, index: number) => (
            <List.Item>
              <List.Item.Meta
                title={
                  <div style={{ fontSize: 13 }}>
                    <Tag color={statusColor[item.status] || 'processing'}>
                      {intl.formatMessage({
                        id: statusKey[item.status] || 'ai.panel.log-running',
                      })}
                    </Tag>
                    {item.created_time
                      ? new Date(item.created_time * 1000).toLocaleString()
                      : ''}
                    {item.tool_calls > 0 &&
                      ` | ${intl.formatMessage(
                        { id: 'ai.panel.tool-call-count' },
                        { count: item.tool_calls },
                      )}`}
                  </div>
                }
                description={
                  <div style={{ fontSize: 12, color: '#666' }}>
                    {item.summary && (
                      <div style={{ marginBottom: 4 }}>
                        {openidx === index ? (
                          <>
                            <Viewer key={`seg-${index}`} value={item.summary} />
                            <Button
                              size="small"
                              onClick={() => setOpenidx(null)}
                            >
                              {intl.formatMessage({ id: 'ai.panel.collapse' })}
                            </Button>
                          </>
                        ) : (
                          <>
                            <div>{item.summary.substring(0, 80)}</div>
                            <Button
                              size="small"
                              onClick={() => setOpenidx(index)}
                            >
                              {intl.formatMessage({ id: 'ai.panel.expand' })}
                            </Button>
                          </>
                        )}
                      </div>
                    )}
                    {item.error && (
                      <div style={{ color: '#ff4d4f' }}>{item.error}</div>
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

export default AgentLogDrawer;
