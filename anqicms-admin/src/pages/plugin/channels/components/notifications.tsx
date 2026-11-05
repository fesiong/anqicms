import {
  channelList,
  channelRuleDelete,
  channelRuleSave,
  channelRules,
  channelTargets,
  channelTemplatePreview,
  channelTemplateSave,
  channelTemplates,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Button, Modal, Space, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

const timeText = (seconds?: number) => {
  if (!seconds || seconds <= 0) {
    return '-';
  }
  return dayjs(seconds * 1000).format('YYYY-MM-DD HH:mm');
};

// 事件类型固定四项，后端只接受这四个常量（R105）
const RULE_EVENTS = [
  'guestbook.created',
  'order.created',
  'order.paid',
  'agent.failed',
];

// 通知规则（含模板）（R105）：规则事件类型只能从固定列表选择；
// 模板语法交由后端 ValidateNotificationTemplate 校验，前端只校验平台覆盖必须是 JSON 对象。
const ChannelNotifications: React.FC = () => {
  const intl = useIntl();
  const ruleActionRef = useRef<ActionType>();
  const templateActionRef = useRef<ActionType>();

  const [targetGroups, setTargetGroups] = useState<any[]>([]);
  const [targetLabels, setTargetLabels] = useState<Record<number, string>>({});
  const [templates, setTemplates] = useState<any[]>([]);

  const [ruleVisible, setRuleVisible] = useState<boolean>(false);
  const [currentRule, setCurrentRule] = useState<any>({});
  const [templateVisible, setTemplateVisible] = useState<boolean>(false);
  const [currentTemplate, setCurrentTemplate] = useState<any>({});
  const [previewVisible, setPreviewVisible] = useState<boolean>(false);
  const [previewTemplate, setPreviewTemplate] = useState<any>({});
  const [previewResult, setPreviewResult] = useState<any>(null);
  const [previewLoading, setPreviewLoading] = useState<boolean>(false);

  const loadTargets = async () => {
    const res = await channelList();
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    const list = Array.isArray(res.data) ? res.data : [];
    const groups: any[] = [];
    const labels: Record<number, string> = {};
    for (const channel of list) {
      const targetRes = await channelTargets({ channel_id: channel.id });
      if (targetRes.code !== 0) {
        message.error(targetRes.msg);
        continue;
      }
      const rows = Array.isArray(targetRes.data) ? targetRes.data : [];
      groups.push({
        label: channel.name || String(channel.id),
        options: rows.map((row: any) => {
          labels[row.id] = `${channel.name || channel.id} / ${
            row.platform_target_id
          }`;
          return {
            label: `${row.platform_target_id} (${row.kind})`,
            value: row.id,
          };
        }),
      });
    }
    setTargetGroups(groups);
    setTargetLabels(labels);
  };

  const loadTemplates = async () => {
    const res = await channelTemplates();
    if (res.code !== 0) {
      message.error(res.msg);
      return;
    }
    setTemplates(Array.isArray(res.data) ? res.data : []);
  };

  useEffect(() => {
    loadTargets();
    loadTemplates();
  }, []);

  const eventLabel = (eventType: string) =>
    intl.formatMessage({ id: `plugin.channels.rule.event.${eventType}` });

  const templateName = (templateId: number) => {
    if (!templateId) {
      return intl.formatMessage({
        id: 'plugin.channels.rule.template.default',
      });
    }
    const template = templates.find((item) => item.id === templateId);
    return template?.name || String(templateId);
  };

  const saveRule = async (payload: any) => {
    const res = await channelRuleSave({
      id: payload.id || 0,
      name: payload.name || '',
      event_type: payload.event_type,
      enabled: payload.enabled || 0,
      target_ids: payload.target_ids || [],
      template_id: payload.template_id || 0,
    });
    if (res.code !== 0) {
      message.error(res.msg);
      return false;
    }
    message.success(res.msg);
    ruleActionRef.current?.reload();
    return true;
  };

  const onSaveRule = async (values: any) => {
    const ok = await saveRule({
      ...currentRule,
      name: values.name,
      event_type: values.event_type,
      enabled: values.enabled,
      target_ids: values.target_ids,
      template_id: values.template_id,
    });
    if (!ok) {
      return false;
    }
    setRuleVisible(false);
    return true;
  };

  const openRuleCreate = () => {
    setCurrentRule({
      id: 0,
      name: '',
      event_type: RULE_EVENTS[0],
      enabled: 1,
      target_ids: [],
      template_id: 0,
    });
    setRuleVisible(true);
  };

  const openRuleEdit = (record: any) => {
    setCurrentRule(record);
    setRuleVisible(true);
  };

  const handleRuleDelete = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.channels.rule.delete.confirm' }),
      onOk: async () => {
        const res = await channelRuleDelete({ id: record.id });
        if (res.code !== 0) {
          message.error(res.msg);
          return;
        }
        message.success(res.msg);
        ruleActionRef.current?.reload();
      },
    });
  };

  const onSaveTemplate = async (values: any) => {
    // 模板语法不做前端校验：后端 ValidateNotificationTemplate 是唯一真源
    const res = await channelTemplateSave({
      id: currentTemplate.id || 0,
      key: values.key,
      name: values.name || '',
      subject: values.subject || '',
      content: values.content || '',
      platform_overrides_json: (values.platform_overrides_json || '').trim(),
    });
    if (res.code !== 0) {
      message.error(res.msg);
      return false;
    }
    message.success(res.msg);
    setTemplateVisible(false);
    templateActionRef.current?.reload();
    loadTemplates();
    return true;
  };

  const openTemplateCreate = () => {
    setCurrentTemplate({
      id: 0,
      key: '',
      name: '',
      subject: '',
      content: '',
      platform_overrides_json: '',
    });
    setTemplateVisible(true);
  };

  const openTemplateEdit = (record: any) => {
    setCurrentTemplate(record);
    setTemplateVisible(true);
  };

  const openPreview = (record: any) => {
    setPreviewTemplate(record);
    setPreviewResult(null);
    setPreviewVisible(true);
  };

  const onPreview = async (values: any) => {
    const payloadJson = (values.payload_json || '').trim();
    setPreviewLoading(true);
    const res = await channelTemplatePreview({
      id: previewTemplate.id || 0,
      target_kind: values.target_kind,
      payload_json: payloadJson,
    });
    setPreviewLoading(false);
    if (res.code !== 0) {
      message.error(res.msg);
      return false;
    }
    // 预览不落库：结果只在弹窗里展示
    setPreviewResult(res.data || null);
    return false;
  };

  const overridesValidator = async (_: any, value: string) => {
    const raw = (value || '').trim();
    if (!raw) {
      return;
    }
    let parsed: any = null;
    try {
      parsed = JSON.parse(raw);
    } catch (e) {
      throw new Error(
        intl.formatMessage({
          id: 'plugin.channels.template.overrides.invalid',
        }),
      );
    }
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error(
        intl.formatMessage({
          id: 'plugin.channels.template.overrides.invalid',
        }),
      );
    }
  };

  const ruleColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.rule.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.rule.event' }),
      dataIndex: 'event_type',
      render: (_, record) => <Tag>{eventLabel(record.event_type)}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.rule.targets' }),
      dataIndex: 'target_ids',
      render: (_, record) =>
        (record.target_ids || []).length > 0
          ? (record.target_ids || []).map((item: number) => (
              <Tag key={item}>{targetLabels[item] || String(item)}</Tag>
            ))
          : '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.rule.template' }),
      dataIndex: 'template_id',
      render: (_, record) => templateName(record.template_id),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.rule.enabled' }),
      dataIndex: 'enabled',
      render: (_, record) => (
        <Tag color={record.enabled === 1 ? 'green' : undefined}>
          {record.enabled === 1
            ? intl.formatMessage({ id: 'plugin.channels.yes' })
            : intl.formatMessage({ id: 'plugin.channels.no' })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.action' }),
      dataIndex: 'action',
      render: (_, record) => (
        <Space size={4} wrap>
          <Button type="link" size="small" onClick={() => openRuleEdit(record)}>
            {intl.formatMessage({ id: 'plugin.channels.action.edit' })}
          </Button>
          <Button
            type="link"
            size="small"
            danger
            onClick={() => handleRuleDelete(record)}
          >
            {intl.formatMessage({ id: 'plugin.channels.action.delete' })}
          </Button>
        </Space>
      ),
    },
  ];

  const templateColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.template.key' }),
      dataIndex: 'key',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.template.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.template.subject' }),
      dataIndex: 'subject',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.template.content' }),
      dataIndex: 'content',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.template.overrides' }),
      dataIndex: 'platform_overrides_json',
      ellipsis: true,
      render: (_, record) => record.platform_overrides_json || '-',
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.created-time' }),
      dataIndex: 'created_time',
      render: (_, record) => timeText(record.created_time),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.table.action' }),
      dataIndex: 'action',
      render: (_, record) => (
        <Space size={4} wrap>
          <Button
            type="link"
            size="small"
            onClick={() => openTemplateEdit(record)}
          >
            {intl.formatMessage({ id: 'plugin.channels.action.edit' })}
          </Button>
          <Button type="link" size="small" onClick={() => openPreview(record)}>
            {intl.formatMessage({ id: 'plugin.channels.template.preview' })}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <>
      <ProTable<any>
        rowKey="id"
        search={false}
        actionRef={ruleActionRef}
        pagination={false}
        headerTitle={intl.formatMessage({ id: 'plugin.channels.rules' })}
        toolBarRender={() => [
          <Button key="add" type="primary" onClick={() => openRuleCreate()}>
            {intl.formatMessage({ id: 'plugin.channels.rule.add' })}
          </Button>,
        ]}
        request={async () => {
          const res = await channelRules();
          if (res.code !== 0) {
            message.error(res.msg);
            return { data: [], success: false, total: 0 };
          }
          const list = Array.isArray(res.data) ? res.data : [];
          return { data: list, success: true, total: list.length };
        }}
        columns={ruleColumns}
      />
      <ProTable<any>
        rowKey="id"
        search={false}
        actionRef={templateActionRef}
        pagination={false}
        headerTitle={intl.formatMessage({ id: 'plugin.channels.templates' })}
        toolBarRender={() => [
          <Button key="add" type="primary" onClick={() => openTemplateCreate()}>
            {intl.formatMessage({ id: 'plugin.channels.template.add' })}
          </Button>,
        ]}
        request={async () => {
          const res = await channelTemplates();
          if (res.code !== 0) {
            message.error(res.msg);
            return { data: [], success: false, total: 0 };
          }
          const list = Array.isArray(res.data) ? res.data : [];
          return { data: list, success: true, total: list.length };
        }}
        columns={templateColumns}
      />
      {ruleVisible && (
        <ModalForm
          width={640}
          title={
            currentRule.id
              ? intl.formatMessage({ id: 'plugin.channels.rule.edit' })
              : intl.formatMessage({ id: 'plugin.channels.rule.add' })
          }
          open={ruleVisible}
          layout="horizontal"
          labelCol={{ span: 6 }}
          initialValues={{
            name: currentRule.name || '',
            event_type: currentRule.event_type || RULE_EVENTS[0],
            enabled: currentRule.enabled === 0 ? 0 : 1,
            target_ids: currentRule.target_ids || [],
            template_id: currentRule.template_id || 0,
          }}
          onOpenChange={(flag) => {
            setRuleVisible(flag);
          }}
          onFinish={onSaveRule}
        >
          <ProFormText
            name="name"
            label={intl.formatMessage({ id: 'plugin.channels.rule.name' })}
          />
          <ProFormSelect
            name="event_type"
            label={intl.formatMessage({ id: 'plugin.channels.rule.event' })}
            options={RULE_EVENTS.map((item) => ({
              label: eventLabel(item),
              value: item,
            }))}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.field.required',
                }),
              },
            ]}
          />
          <ProFormSelect
            name="target_ids"
            label={intl.formatMessage({ id: 'plugin.channels.rule.targets' })}
            options={targetGroups}
            fieldProps={{ mode: 'multiple' }}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.rule.targets.required',
                }),
              },
            ]}
          />
          <ProFormSelect
            name="template_id"
            label={intl.formatMessage({ id: 'plugin.channels.rule.template' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.channels.rule.template.default',
                }),
                value: 0,
              },
              ...templates.map((item) => ({
                label: item.name || item.key,
                value: item.id,
              })),
            ]}
          />
          <ProFormRadio.Group
            name="enabled"
            label={intl.formatMessage({ id: 'plugin.channels.rule.enabled' })}
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.channels.yes' }),
                value: 1,
              },
              {
                label: intl.formatMessage({ id: 'plugin.channels.no' }),
                value: 0,
              },
            ]}
          />
        </ModalForm>
      )}
      {templateVisible && (
        <ModalForm
          width={720}
          title={
            currentTemplate.id
              ? intl.formatMessage({ id: 'plugin.channels.template.edit' })
              : intl.formatMessage({ id: 'plugin.channels.template.add' })
          }
          open={templateVisible}
          layout="horizontal"
          labelCol={{ span: 6 }}
          initialValues={{
            key: currentTemplate.key || '',
            name: currentTemplate.name || '',
            subject: currentTemplate.subject || '',
            content: currentTemplate.content || '',
            platform_overrides_json:
              currentTemplate.platform_overrides_json || '',
          }}
          onOpenChange={(flag) => {
            setTemplateVisible(flag);
          }}
          onFinish={onSaveTemplate}
        >
          <ProFormText
            name="key"
            label={intl.formatMessage({ id: 'plugin.channels.template.key' })}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.field.required',
                }),
              },
            ]}
          />
          <ProFormText
            name="name"
            label={intl.formatMessage({ id: 'plugin.channels.template.name' })}
          />
          <ProFormText
            name="subject"
            label={intl.formatMessage({ id: 'plugin.channels.template.subject' })}
          />
          <ProFormTextArea
            name="content"
            label={intl.formatMessage({ id: 'plugin.channels.template.content' })}
            extra={intl.formatMessage({
              id: 'plugin.channels.template.content.description',
            })}
            fieldProps={{ rows: 8 }}
          />
          <ProFormTextArea
            name="platform_overrides_json"
            label={intl.formatMessage({
              id: 'plugin.channels.template.overrides',
            })}
            fieldProps={{ rows: 4 }}
            rules={[{ validator: overridesValidator }]}
          />
        </ModalForm>
      )}
      {previewVisible && (
        <ModalForm
          width={640}
          title={intl.formatMessage({
            id: 'plugin.channels.template.preview.title',
          })}
          open={previewVisible}
          layout="horizontal"
          labelCol={{ span: 6 }}
          initialValues={{ target_kind: 'group', payload_json: '' }}
          submitter={{
            searchConfig: {
              submitText: intl.formatMessage({
                id: 'plugin.channels.template.preview',
              }),
            },
            submitButtonProps: { loading: previewLoading },
          }}
          onOpenChange={(flag) => {
            setPreviewVisible(flag);
            if (!flag) {
              setPreviewResult(null);
            }
          }}
          onFinish={onPreview}
        >
          <ProFormSelect
            name="target_kind"
            label={intl.formatMessage({
              id: 'plugin.channels.template.preview.target-kind',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.channels.target.kind.group',
                }),
                value: 'group',
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.channels.target.kind.private',
                }),
                value: 'private',
              },
            ]}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'plugin.channels.field.required',
                }),
              },
            ]}
          />
          <ProFormTextArea
            name="payload_json"
            label={intl.formatMessage({
              id: 'plugin.channels.template.preview.payload',
            })}
            fieldProps={{ rows: 4 }}
          />
          {previewResult && (
            <Space direction="vertical" size={4} style={{ width: '100%' }}>
              <span>
                {intl.formatMessage({
                  id: 'plugin.channels.template.preview.result',
                })}
              </span>
              <div>{previewResult.subject || '-'}</div>
              <div style={{ whiteSpace: 'pre-wrap' }}>
                {previewResult.text || '-'}
              </div>
            </Space>
          )}
        </ModalForm>
      )}
    </>
  );
};

export default ChannelNotifications;
