import { useIntl } from '@umijs/max';
import { Button, Drawer, List, Popconfirm, Spin, Tag, Tooltip, message } from 'antd';
import {
  ReloadOutlined,
  DeleteOutlined,
  EyeOutlined,
  FileTextOutlined,
} from '@ant-design/icons';

interface SkillItem {
  name: string;
  description: string;
  category: string;
  version: string;
  tags: string[];
  updated_at: string;
  file_count: number;
}

interface SkillListDrawerProps {
  visible: boolean;
  loading: boolean;
  skills: SkillItem[];
  onClose: () => void;
  onView: (skill: SkillItem) => void;
  onReload: () => void;
  onDelete: (skill: SkillItem) => void;
}

const SkillListDrawer: React.FC<SkillListDrawerProps> = ({
  visible,
  loading,
  skills,
  onClose,
  onView,
  onReload,
  onDelete,
}) => {
  const intl = useIntl();
  return (
    <Drawer
      title={intl.formatMessage({ id: 'ai.panel.skill-title' })}
      placement="right"
      className="skill-list-drawer"
      open={visible}
      onClose={onClose}
      width={560}
      extra={
        <Tooltip title={intl.formatMessage({ id: 'ai.panel.skill-reload' })}>
          <Button
            type="text"
            icon={<ReloadOutlined />}
            onClick={onReload}
          />
        </Tooltip>
      }
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: '40px 0' }}>
          <Spin size="small" />
        </div>
      ) : skills.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
          {intl.formatMessage({ id: 'ai.panel.no-skills' })}
          <br />
          {intl.formatMessage({ id: 'ai.panel.create-hint' })}
          <br />
          <em>
            {intl.formatMessage({ id: 'ai.panel.skill-create-example' })}
          </em>
        </div>
      ) : (
        <List
          size="small"
          dataSource={skills}
          renderItem={(item: SkillItem) => (
            <List.Item
              actions={[
                <Button
                  key="view"
                  type="link"
                  size="small"
                  icon={<EyeOutlined />}
                  onClick={() => onView(item)}
                >
                  {intl.formatMessage({ id: 'ai.panel.view' })}
                </Button>,
                <Popconfirm
                  key="delete"
                  title={intl.formatMessage(
                    { id: 'ai.panel.skill-delete-confirm' },
                    { name: item.name },
                  )}
                  onConfirm={() => onDelete(item)}
                  okText={intl.formatMessage({ id: 'ai.panel.confirm' })}
                  cancelText={intl.formatMessage({ id: 'ai.panel.cancel' })}
                >
                  <Button
                    type="link"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                  >
                    {intl.formatMessage({ id: 'ai.settings.delete' })}
                  </Button>
                </Popconfirm>,
              ]}
            >
              <List.Item.Meta
                title={
                  <div style={{ fontSize: 13, fontWeight: 500 }}>
                    <FileTextOutlined style={{ marginRight: 6 }} />
                    {item.name}
                    {item.version && (
                      <Tag style={{ marginLeft: 6, fontSize: 11 }}>
                        v{item.version}
                      </Tag>
                    )}
                    {item.category && (
                      <Tag color="blue" style={{ fontSize: 11 }}>
                        {item.category}
                      </Tag>
                    )}
                  </div>
                }
                description={
                  <div style={{ fontSize: 12, color: '#666' }}>
                    {item.description && (
                      <div
                        style={{
                          marginBottom: 4,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {item.description}
                      </div>
                    )}
                    <div style={{ color: '#999' }}>
                      {item.tags && item.tags.length > 0 && (
                        <span>
                          {item.tags.map((tag) => (
                            <Tag key={tag} style={{ fontSize: 10 }}>
                              {tag}
                            </Tag>
                          ))}
                        </span>
                      )}
                      <span>
                        {intl.formatMessage(
                          { id: 'ai.panel.file-count' },
                          { count: item.file_count },
                        )}
                        {item.updated_at
                          ? ` | ${intl.formatMessage(
                              { id: 'ai.panel.updated-at' },
                              { time: item.updated_at },
                            )}`
                          : ''}
                      </span>
                    </div>
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

export default SkillListDrawer;
