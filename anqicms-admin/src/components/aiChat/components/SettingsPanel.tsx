import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Button, List, Modal } from 'antd';
import { AiProviderConfig } from '../types';

interface SettingsPanelProps {
  visible: boolean;
  onClose: () => void;
  customProviders: AiProviderConfig[];
  onAdd: () => void;
  onEdit: (index: number, provider: AiProviderConfig) => void;
  onDelete: (index: number) => void;
}

const SettingsPanel: React.FC<SettingsPanelProps> = ({
  visible,
  onClose,
  customProviders,
  onAdd,
  onEdit,
  onDelete,
}) => {
  const intl = useIntl();
  return (
    <Modal
      title={intl.formatMessage({ id: 'ai.settings.title' })}
      open={visible}
      onCancel={onClose}
      footer={null}
      width={560}
    >
      <div style={{ marginBottom: 12 }}>
        <Button type="primary" size="small" onClick={onAdd}>
          {intl.formatMessage({ id: 'ai.settings.add-provider' })}
        </Button>
      </div>
      {customProviders.length === 0 ? (
        <div style={{ textAlign: 'center', padding: '24px 0', color: '#999' }}>
          {intl.formatMessage({ id: 'ai.settings.no-providers' })}
        </div>
      ) : (
        <List
          size="small"
          dataSource={customProviders}
          renderItem={(item: AiProviderConfig, index: number) => (
            <List.Item
              actions={[
                <Button
                  key="edit"
                  type="link"
                  size="small"
                  onClick={() => onEdit(index, item)}
                >
                  {intl.formatMessage({ id: 'ai.settings.edit' })}
                </Button>,
                <Button
                  key="delete"
                  type="link"
                  size="small"
                  danger
                  onClick={() => onDelete(index)}
                >
                  {intl.formatMessage({ id: 'ai.settings.delete' })}
                </Button>,
              ]}
            >
              <List.Item.Meta
                title={item.name}
                description={
                  <span style={{ fontSize: 12, color: '#999' }}>
                    {item.base_url} | {item.model}
                  </span>
                }
              />
            </List.Item>
          )}
        />
      )}
    </Modal>
  );
};

export default SettingsPanel;

// -------------------------------------------------------------------------
// 编辑自定义接口表单
// -------------------------------------------------------------------------

interface ProviderFormModalProps {
  visible: boolean;
  editProvider: AiProviderConfig | null;
  editIndex: number;
  onClose: () => void;
  onSave: (values: any) => void;
}

export const ProviderFormModal: React.FC<ProviderFormModalProps> = ({
  visible,
  editProvider,
  editIndex,
  onClose,
  onSave,
}) => {
  const intl = useIntl();
  return (
    <ModalForm
      title={
        editIndex !== -1
          ? intl.formatMessage({ id: 'ai.settings.edit-provider' })
          : intl.formatMessage({ id: 'ai.settings.add-provider' })
      }
      open={visible}
      onOpenChange={(flag) => {
        if (!flag) onClose();
      }}
      layout="horizontal"
      initialValues={editProvider || {}}
      onFinish={async (values) => {
        onSave(values);
      }}
      width={520}
    >
      {editProvider && (
        <div>
          <ProFormText
            name="name"
            label={intl.formatMessage({ id: 'ai.settings.provider-name' })}
            placeholder={intl.formatMessage({
              id: 'ai.settings.provider-name-placeholder',
            })}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'ai.settings.provider-name-required',
                }),
              },
            ]}
          />
          <ProFormText
            name="base_url"
            label={intl.formatMessage({ id: 'ai.settings.base-url' })}
            placeholder="https://api.openai.com/v1"
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'ai.settings.base-url-required',
                }),
              },
            ]}
          />
          <ProFormText
            name="api_key"
            label="API Key"
            placeholder="sk-xxxxxxxxxxxxxxxx"
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'ai.settings.api-key-required',
                }),
              },
            ]}
          />
          <ProFormText
            name="model"
            label={intl.formatMessage({ id: 'ai.settings.model' })}
            placeholder="deepseek-v4-flash"
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'ai.settings.model-required',
                }),
              },
            ]}
          />
          <ProFormRadio.Group
            name="enable_reasoning"
            label={intl.formatMessage({ id: 'ai.settings.reasoning-mode' })}
            options={[
              {
                label: intl.formatMessage({ id: 'ai.settings.enable' }),
                value: true,
              },
              {
                label: intl.formatMessage({ id: 'ai.settings.disable' }),
                value: false,
              },
            ]}
          />
          <ProFormDigit
            name="max_tokens"
            label={intl.formatMessage({ id: 'ai.settings.max-tokens' })}
            placeholder="8192"
          />
          <ProFormDigit
            name="timeout_seconds"
            label={intl.formatMessage({ id: 'ai.settings.timeout' })}
            placeholder="120"
          />
        </div>
      )}
    </ModalForm>
  );
};
