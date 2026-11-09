import { channelCapabilities } from '@/services';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Input,
  Modal,
  Segmented,
  Space,
  Table,
  Tag,
  message,
} from 'antd';
import React, { useEffect, useMemo, useState } from 'react';
import {
  applyPolicyPreset,
  buildPolicyRows,
  serializePolicy,
} from '../policy';
import type {
  CapabilityIntent,
  PolicyDecision,
  PolicyPreset,
  PolicyRow,
} from '../policy';

type Props = {
  open: boolean;
  agentId?: number;
  value: string;
  onCancel: () => void;
  onApply: (json: string) => void;
};

const RISK_COLORS: Record<string, string> = {
  read: 'green',
  write: 'orange',
  destructive: 'red',
  system: 'magenta',
};

const KNOWN_RISKS = ['read', 'write', 'destructive', 'system'];

// 能力策略编辑器：把「意图 + 动作」逐项映射成 拒绝 / 需确认 / 允许 三态，
// 保存时序列化成与后端一致的 policy_json。界面不暴露 JSON，也不提供手写入口。
const ChannelPolicyEditor: React.FC<Props> = ({
  open,
  agentId,
  value,
  onCancel,
  onApply,
}) => {
  const intl = useIntl();
  const [rows, setRows] = useState<PolicyRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);

  // 只在打开时拉取能力并回填：编辑过程中的三态不应被父组件重渲染冲掉。
  useEffect(() => {
    if (!open) {
      return;
    }
    setKeyword('');
    setPage(1);
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const res = await channelCapabilities(
          agentId ? { agent_id: agentId } : {},
        );
        if (cancelled) {
          return;
        }
        if (res.code !== 0) {
          message.error(res.msg);
          setRows([]);
          return;
        }
        const intents: CapabilityIntent[] = Array.isArray(res.data?.intents)
          ? res.data.intents
          : [];
        setRows(buildPolicyRows(intents, value));
      } catch (e) {
        if (!cancelled) {
          message.error(
            intl.formatMessage({ id: 'plugin.channels.policy.load-failed' }),
          );
          setRows([]);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };
    load();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    setPage(1);
  }, [keyword]);

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase();
    if (kw === '') {
      return rows;
    }
    return rows.filter((row) =>
      [row.intent, row.title].some((item) =>
        String(item || '')
          .toLowerCase()
          .includes(kw),
      ),
    );
  }, [rows, keyword]);

  const setDecision = (key: string, decision: PolicyDecision) => {
    setRows((prev) =>
      prev.map((row) => (row.key === key ? { ...row, decision } : row)),
    );
  };

  const applyPreset = (preset: PolicyPreset) => {
    setRows((prev) => applyPolicyPreset(prev, preset));
  };

  const decisionOptions = [
    {
      label: intl.formatMessage({ id: 'plugin.channels.policy.decision.deny' }),
      value: 'deny',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.channels.policy.decision.confirm',
      }),
      value: 'confirm',
    },
    {
      label: intl.formatMessage({ id: 'plugin.channels.policy.decision.allow' }),
      value: 'allow',
    },
  ];

  const riskLabel = (risk: string) => {
    if (!KNOWN_RISKS.includes(risk)) {
      return risk || '-';
    }
    return intl.formatMessage({ id: `plugin.channels.policy.risk.${risk}` });
  };

  const columns = [
    {
      title: intl.formatMessage({ id: 'plugin.channels.policy.column.intent' }),
      dataIndex: 'intent',
      render: (_: any, row: PolicyRow) => (
        <Space size={4} wrap>
          <span>{row.title}</span>
          <span style={{ color: 'rgba(0,0,0,0.45)', fontSize: 12 }}>
            {row.intent}
          </span>
          <Tag color={RISK_COLORS[row.risk] || undefined}>
            {riskLabel(row.risk)}
          </Tag>
          {row.defaultOff && (
            <Tag title={intl.formatMessage({
              id: 'plugin.channels.policy.default-off.hint',
            })}>
              {intl.formatMessage({
                id: 'plugin.channels.policy.default-off',
              })}
            </Tag>
          )}
          {row.blocked && (
            <Tag color="red" title={row.reason}>
              {intl.formatMessage({ id: 'plugin.channels.policy.blocked' })}
            </Tag>
          )}
          {row.agentAllowed === false && (
            <Tag color="orange">
              {intl.formatMessage({
                id: 'plugin.channels.policy.agent-unallowed',
              })}
            </Tag>
          )}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.channels.policy.column.decision' }),
      dataIndex: 'decision',
      width: 260,
      render: (_: any, row: PolicyRow) => (
        <Segmented
          size="small"
          disabled={row.blocked}
          value={row.decision}
          options={decisionOptions}
          onChange={(next) => setDecision(row.key, next as PolicyDecision)}
        />
      ),
    },
  ];

  return (
    <Modal
      open={open}
      width={880}
      title={intl.formatMessage({ id: 'plugin.channels.policy.title' })}
      maskClosable={false}
      onCancel={onCancel}
      footer={[
        <Button key="cancel" onClick={onCancel}>
          {intl.formatMessage({ id: 'plugin.channels.policy.cancel' })}
        </Button>,
        <Button
          key="apply"
          type="primary"
          disabled={loading}
          onClick={() => onApply(serializePolicy(rows))}
        >
          {intl.formatMessage({ id: 'plugin.channels.policy.apply' })}
        </Button>,
      ]}
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({ id: 'plugin.channels.policy.intro' })}
        />
        <Space wrap>
          <Input
            allowClear
            style={{ width: 260 }}
            placeholder={intl.formatMessage({
              id: 'plugin.channels.policy.search',
            })}
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
          />
          <span style={{ color: 'rgba(0,0,0,0.45)' }}>
            {intl.formatMessage({ id: 'plugin.channels.policy.preset' })}
          </span>
          <Button size="small" onClick={() => applyPreset('deny')}>
            {intl.formatMessage({ id: 'plugin.channels.policy.preset.deny' })}
          </Button>
          <Button size="small" onClick={() => applyPreset('readonly')}>
            {intl.formatMessage({ id: 'plugin.channels.policy.preset.readonly' })}
          </Button>
          <Button size="small" onClick={() => applyPreset('regular')}>
            {intl.formatMessage({ id: 'plugin.channels.policy.preset.regular' })}
          </Button>
          <Button size="small" onClick={() => applyPreset('allow')}>
            {intl.formatMessage({ id: 'plugin.channels.policy.preset.allow' })}
          </Button>
        </Space>
        <Table<PolicyRow>
          rowKey="key"
          size="small"
          loading={loading}
          dataSource={filtered}
          columns={columns}
          pagination={{
            current: page,
            pageSize: 20,
            showSizeChanger: false,
            onChange: (next) => setPage(next),
          }}
        />
      </Space>
    </Modal>
  );
};

export default ChannelPolicyEditor;
