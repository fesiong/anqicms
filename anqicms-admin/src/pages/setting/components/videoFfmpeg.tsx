import {
  getVideoFFmpeg,
  installVideoFFmpeg,
} from '@/services/setting';
import { ReloadOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Input,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const VideoFfmpegPanel: React.FC = () => {
  const intl = useIntl();
  const [status, setStatus] = useState<any>(null);
  const [checking, setChecking] = useState(true);
  const [installing, setInstalling] = useState(false);
  const [stage, setStage] = useState('');
  const [logs, setLogs] = useState<string[]>([]);
  const abortRef = useRef<AbortController>();

  const detect = async () => {
    setChecking(true);
    try {
      const res = await getVideoFFmpeg();
      setStatus(res.data || null);
    } finally {
      setChecking(false);
    }
  };

  useEffect(() => {
    detect();
    return () => abortRef.current?.abort();
  }, []);

  const install = async () => {
    const controller = new AbortController();
    abortRef.current = controller;
    setInstalling(true);
    setStage('detect');
    setLogs([]);
    try {
      await installVideoFFmpeg((event) => {
        setStage(event.stage || '');
        const line = event.output || event.message;
        if (line) {
          setLogs((current) => {
            const next = [...current];
            if (event.command) next.push(`$ ${event.command}`);
            next.push(line);
            return next.slice(-160);
          });
        }
        if (event.stage === 'done' || event.stage === 'error') {
          setInstalling(false);
          detect();
        }
      }, { signal: controller.signal });
    } catch (error: any) {
      if (error.name !== 'AbortError') {
        setInstalling(false);
        setLogs((current) => [...current, error.message || String(error)]);
      }
    }
  };

  const ready = Boolean(status?.ready);
  return (
    <Card
      size="small"
      title={intl.formatMessage({ id: 'setting.video.ffmpeg.title' })}
      extra={
        <Space>
          {status?.can_install && !ready && (
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              loading={installing}
              onClick={install}
            >
              {intl.formatMessage({ id: 'setting.video.ffmpeg.install' })}
            </Button>
          )}
          <Button
            icon={<ReloadOutlined />}
            loading={checking}
            disabled={installing}
            onClick={detect}
          >
            {intl.formatMessage({ id: 'setting.video.ffmpeg.recheck' })}
          </Button>
        </Space>
      }
    >
      {checking && !status ? (
        <Spin tip={intl.formatMessage({ id: 'setting.video.ffmpeg.checking' })} />
      ) : (
        <>
          <Alert
            type={ready ? 'success' : 'warning'}
            showIcon
            message={
              ready
                ? intl.formatMessage({ id: 'setting.video.ffmpeg.ready' })
                : intl.formatMessage({ id: 'setting.video.ffmpeg.missing' })
            }
            description={status?.message}
          />
          <Descriptions
            size="small"
            column={{ xs: 1, sm: 2, lg: 3 }}
            style={{ marginTop: 16 }}
          >
            <Descriptions.Item
              label={intl.formatMessage({ id: 'setting.video.ffmpeg.platform' })}
            >
              <Tag>{status?.platform || '-'}</Tag>
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({ id: 'setting.video.ffmpeg.version' })}
            >
              <Typography.Text ellipsis style={{ maxWidth: 220 }}>
                {status?.version || '-'}
              </Typography.Text>
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({ id: 'setting.video.ffmpeg.path' })}
            >
              <Typography.Text code ellipsis style={{ maxWidth: 260 }}>
                {status?.path || status?.ffprobe_path || '-'}
              </Typography.Text>
            </Descriptions.Item>
          </Descriptions>
          {!status?.can_install && !ready && (
            <Typography.Text type="secondary">
              {intl.formatMessage({ id: 'setting.video.ffmpeg.install.tip' })}
            </Typography.Text>
          )}
        </>
      )}
      {(installing || logs.length > 0) && (
        <div style={{ marginTop: 16 }}>
          <Space style={{ marginBottom: 8 }}>
            {installing && <Spin size="small" />}
            <Typography.Text strong>
              {installing
                ? intl.formatMessage({ id: 'setting.video.ffmpeg.installing' })
                : stage}
            </Typography.Text>
          </Space>
          <Input.TextArea
            value={logs.join('\n')}
            readOnly
            autoSize={{ minRows: 4, maxRows: 10 }}
            aria-label={intl.formatMessage({
              id: 'setting.video.ffmpeg.progress',
            })}
          />
        </div>
      )}
    </Card>
  );
};

export default VideoFfmpegPanel;
