import {
  BugOutlined,
  CloseOutlined,
  CloudServerOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  HistoryOutlined,
  MoreOutlined,
  PlusOutlined,
  SettingOutlined,
  ToolOutlined,
} from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Button, Dropdown, List, Popover, Spin, Tooltip } from 'antd';

interface HeaderProps {
  maximized: boolean;
  onClose: () => void;
  onToggleMaximize: () => void;
  onNewSession: () => void;
  onOpenAgentDrawer: () => void;
  onOpenSettings: () => void;
  onOpenSkillsList: () => void;
  onOpenTrace: () => void;
  sessionListOpen: boolean;
  sessionListLoading: boolean;
  sessionList: any[];
  onOpenSessionList: () => void;
  onSwitchSession: (sid: string) => void;
  onCloseSessionList: () => void;
}

const Header: React.FC<HeaderProps> = ({
  maximized,
  onClose,
  onToggleMaximize,
  onNewSession,
  onOpenAgentDrawer,
  onOpenSettings,
  onOpenSkillsList,
  onOpenTrace,
  sessionListOpen,
  sessionListLoading,
  sessionList,
  onOpenSessionList,
  onSwitchSession,
  onCloseSessionList,
}) => {
  const intl = useIntl();
  return (
    <div className="ai-chat-header">
      <div className="ai-chat-title">
        <svg
          className="ai-chat-icon"
          viewBox="0 0 1024 1024"
          version="1.1"
          xmlns="http://www.w3.org/2000/svg"
          width="20"
          height="20"
        >
          <path
            d="M521.47 859.38c-63.86 0-127.41-8.29-189.15-24.55h-125.9c-15.16 0.1-29.61-6.37-39.51-17.78s-14.35-26.57-12.23-41.63l14.25-100.74c-22.94-37.49-36.48-96.5-40.32-176.02-10.1-183.8 109.23-349.81 286.76-398.71 39-9.9 79.22-14.85 119.43-14.65 118.52 0 217.64 37.08 286.56 107.51s107.71 176.52 106.09 294.64c-0.3 57.39-12.93 114.08-37.08 166.11-80.22 167.63-239.97 205.62-368.9 205.82z m-308.48-82.35h121.15c4.75 0 9.4 0.61 13.94 1.92 38.6 10.61 372.24 96.5 489.75-149.54 20.61-44.66 31.42-93.26 31.83-142.37 1.92-102.46-29.91-192.99-89.52-253.01s-140.45-90.74-245.23-90.74c-35.36-0.2-70.63 4.04-104.98 12.73-150.75 41.93-252 183.19-243.31 339.4 3.23 70.02 14.65 121.96 32.84 149.95 6.47 9.7 9.19 21.42 7.48 33.04l-13.95 98.62z"
            fill="#ffffff"
          ></path>
          <path
            d="M540.57 574.74l-13.14-34.76H415.18l-13.34 35.26c-3.23 9.8-7.58 19.2-13.14 27.99-4.34 5.25-11.11 7.98-17.99 7.38-7.17 0.2-14.25-2.63-19.3-7.68a23.851 23.851 0 0 1-8.08-17.58c0.1-3.94 0.71-7.78 1.92-11.62 1.31-4.04 3.23-9.7 6.16-16.77l70.83-179.45c1.92-5.25 4.45-11.42 7.17-19.3 2.32-6.37 5.36-12.33 9.09-17.99 3.23-4.85 7.48-8.79 12.53-11.62 17.48-10.41 40.01-5.36 51.53 11.42 3.23 4.65 5.96 9.6 8.08 14.85l8.49 21.42 72.25 178.44c5.05 9.4 8.29 19.7 9.7 30.31-0.1 6.77-2.93 13.14-7.88 17.78-5.05 5.15-12.02 7.98-19.3 7.88-3.94 0.1-7.78-0.71-11.42-2.32-3.03-1.52-5.76-3.64-7.88-6.37-2.83-3.84-5.05-8.08-6.77-12.53-3.32-5.44-4.73-10.49-7.26-14.74z m-110.74-77.19h82.35l-41.53-114.08-40.82 114.08z m213.8 78.91V358.82a36.171 36.171 0 0 1 7.68-25.26c4.95-5.46 11.92-8.59 19.3-8.49 7.68-0.3 15.16 2.73 20.41 8.29 5.66 7.17 8.49 16.27 7.68 25.46v217.64c0.81 9.19-2.02 18.39-7.68 25.66-5.25 5.66-12.73 8.79-20.41 8.49-7.38 0-14.35-3.03-19.3-8.49-5.66-7.27-8.39-16.36-7.68-25.66z"
            fill="#ffffff"
          ></path>
        </svg>
        <span>{intl.formatMessage({ id: 'ai.panel.assistant' })}</span>
      </div>
      <div
        className="ai-chat-header-actions"
        style={{ display: 'flex', gap: 4, alignItems: 'center' }}
      >
        <Tooltip title={intl.formatMessage({ id: 'ai.panel.new-session' })}>
          <Button type="text" icon={<PlusOutlined />} onClick={onNewSession} />
        </Tooltip>
        <Popover
          open={sessionListOpen}
          onOpenChange={(open) => {
            if (!open) onCloseSessionList();
          }}
          trigger="click"
          placement="bottomRight"
          content={
            <div style={{ width: 260, maxHeight: 320, overflowY: 'auto' }}>
              {sessionListLoading ? (
                <div style={{ textAlign: 'center', padding: '16px 0' }}>
                  <Spin size="small" />
                </div>
              ) : sessionList.length === 0 ? (
                <div
                  style={{
                    textAlign: 'center',
                    padding: '16px 0',
                    color: '#999',
                  }}
                >
                  {intl.formatMessage({ id: 'ai.panel.no-history' })}
                </div>
              ) : (
                <List
                  size="small"
                  dataSource={sessionList}
                  renderItem={(item: any) => (
                    <List.Item
                      onClick={() =>
                        onSwitchSession(item.session_id || item.id)
                      }
                      style={{
                        cursor: 'pointer',
                        padding: '8px 12px',
                        borderRadius: 4,
                      }}
                      onMouseEnter={(e) => {
                        (e.currentTarget as HTMLElement).style.background =
                          '#f5f5f5';
                      }}
                      onMouseLeave={(e) => {
                        (e.currentTarget as HTMLElement).style.background =
                          'transparent';
                      }}
                    >
                      <List.Item.Meta
                        title={
                          <div
                            style={{
                              fontSize: 13,
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {item.title ||
                              intl.formatMessage(
                                { id: 'ai.panel.session-title' },
                                {
                                  id: (item.session_id || item.id || '').slice(
                                    0,
                                    8,
                                  ),
                                },
                              )}
                          </div>
                        }
                        description={
                          <span style={{ fontSize: 12, color: '#999' }}>
                            {item.created_time
                              ? new Date(
                                  item.created_time * 1000,
                                ).toLocaleString()
                              : ''}
                          </span>
                        }
                      />
                    </List.Item>
                  )}
                />
              )}
            </div>
          }
        >
          <Tooltip
            title={intl.formatMessage({ id: 'ai.panel.history-session' })}
          >
            <Button
              type="text"
              icon={<HistoryOutlined />}
              onClick={onOpenSessionList}
            />
          </Tooltip>
        </Popover>
        <Dropdown
          menu={{
            items: [
              {
                key: 'agents',
                icon: <CloudServerOutlined />,
                label: intl.formatMessage({ id: 'ai.panel.agents' }),
                onClick: onOpenAgentDrawer,
              },
              {
                key: 'skills',
                icon: <ToolOutlined />,
                label: intl.formatMessage({ id: 'ai.panel.skills' }),
                onClick: onOpenSkillsList,
              },
              {
                key: 'settings',
                icon: <SettingOutlined />,
                label: intl.formatMessage({ id: 'ai.settings.title' }),
                onClick: onOpenSettings,
              },
              {
                key: 'trace',
                icon: <BugOutlined />,
                label: intl.formatMessage({ id: 'ai.panel.open-trace' }),
                onClick: onOpenTrace,
              },
            ],
          }}
          key="ai-more"
        >
          <Button type="text">
            <MoreOutlined />
          </Button>
        </Dropdown>
        <Tooltip
          title={
            maximized
              ? intl.formatMessage({ id: 'ai.panel.restore' })
              : intl.formatMessage({ id: 'ai.panel.maximize' })
          }
        >
          <Button
            type="text"
            icon={
              maximized ? <FullscreenExitOutlined /> : <FullscreenOutlined />
            }
            onClick={onToggleMaximize}
          />
        </Tooltip>
        <Button
          type="text"
          icon={<CloseOutlined />}
          onClick={onClose}
          className="ai-chat-close"
        />
      </div>
    </div>
  );
};

export default Header;
