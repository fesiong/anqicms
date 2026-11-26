import VideoPlayer from '@/components/VideoPlayer';
import {
  enqueueHistoricalVideoTasks,
  getVideoTasks,
  retryVideoTask,
} from '@/services/setting';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Modal,
  Progress,
  Space,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

const statusColors: Record<number, string> = {
  0: 'default',
  1: 'processing',
  2: 'success',
  3: 'error',
  4: 'warning',
};

const VideoTasks: React.FC = () => {
  const intl = useIntl();
  const actionRef = useRef<ActionType>();
  const [hasActiveTasks, setHasActiveTasks] = useState(false);
  const [retryingId, setRetryingId] = useState<number | null>(null);
  const [preview, setPreview] = useState<any>(null);
  const [playbackError, setPlaybackError] = useState(false);

  useEffect(() => {
    setPlaybackError(false);
  }, [preview]);

  useEffect(() => {
    if (!hasActiveTasks) return;
    const timer = window.setInterval(() => actionRef.current?.reload(), 2500);
    return () => window.clearInterval(timer);
  }, [hasActiveTasks]);

  const statusLabel = (status: number) =>
    intl.formatMessage({
      id: `setting.video.tasks.status.${
        ['pending', 'processing', 'success', 'failed', 'cancelled'][status] ||
        'pending'
      }`,
    });

  const requestTasks = async (params: any) => {
    const res = await getVideoTasks(params);
    const data = res.data || [];
    setHasActiveTasks(
      data.some((task: any) => task.status === 0 || task.status === 1),
    );
    return { data, total: res.total || 0, success: res.code === 0 };
  };

  const handleRetry = async (id: number) => {
    setRetryingId(id);
    try {
      const res = await retryVideoTask(id);
      if (res.code === 0) {
        message.success(res.msg);
        actionRef.current?.reload();
      } else {
        message.error(
          res.msg ||
            intl.formatMessage({ id: 'setting.video.tasks.retry.failed' }),
        );
      }
    } catch (error) {
      message.error(
        error instanceof Error
          ? error.message
          : intl.formatMessage({ id: 'setting.video.tasks.retry.failed' }),
      );
    } finally {
      setRetryingId(null);
    }
  };

  const handleBatch = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.video.tasks.batch.confirm' }),
      content: intl.formatMessage({ id: 'setting.video.tasks.batch.content' }),
      onOk: async () => {
        try {
          const res = await enqueueHistoricalVideoTasks(false);
          if (res.code === 0) {
            message.success(
              intl.formatMessage(
                { id: 'setting.video.tasks.batch.result' },
                {
                  created: String(res.data?.created || 0),
                  retried: String(res.data?.retried || 0),
                  skipped: String(res.data?.skipped || 0),
                },
              ),
            );
            actionRef.current?.reload();
          } else {
            message.error(
              res.msg ||
                intl.formatMessage({ id: 'setting.video.tasks.batch.failed' }),
            );
          }
        } catch (error) {
          message.error(
            error instanceof Error
              ? error.message
              : intl.formatMessage({ id: 'setting.video.tasks.batch.failed' }),
          );
        }
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'setting.video.tasks.id' }),
      dataIndex: 'id',
      width: 80,
      search: false,
    },
    {
      title: intl.formatMessage({ id: 'setting.video.tasks.attachment' }),
      dataIndex: 'attachment',
      search: false,
      render: (_, record) => (
        <div>
          {record.attachment?.play_url || record.attachment?.file_path ? (
            <a
              href={record.attachment.play_url || record.attachment.file_path}
              target="_blank"
              rel="noreferrer"
            >
              {record.attachment.file_name || `#${record.attachment_id}`}
            </a>
          ) : (
            <Typography.Text type="secondary">
              {record.attachment?.file_name || `#${record.attachment_id}`}
            </Typography.Text>
          )}
          {record.attachment?.width > 0 && (
            <div className="text-muted">
              {record.attachment.width} × {record.attachment.height}
            </div>
          )}
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.video.tasks.status' }),
      dataIndex: 'status',
      valueType: 'select',
      valueEnum: Object.fromEntries(
        [0, 1, 2, 3, 4].map((status) => [
          status,
          { text: statusLabel(status) },
        ]),
      ),
      render: (_, record) => (
        <Tag color={statusColors[record.status]}>
          {statusLabel(record.status)}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.video.tasks.progress' }),
      dataIndex: 'progress',
      search: false,
      width: 150,
      render: (_, record) => (
        <Progress
          percent={record.progress || 0}
          size="small"
          status={record.status === 3 ? 'exception' : 'normal'}
        />
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.video.tasks.config' }),
      dataIndex: 'config',
      search: false,
      render: (_, record) => (
        <div>
          <Tag>{String(record.config?.format || '-').toUpperCase()}</Tag>
          <div className="text-muted">
            {(record.config?.qualities || []).join(' / ') || '-'}
          </div>
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.video.tasks.error' }),
      dataIndex: 'error_msg',
      search: false,
      ellipsis: true,
      render: (text) =>
        text ? (
          <Tooltip title={text}>
            <Typography.Text type="danger" ellipsis style={{ maxWidth: 220 }}>
              {text}
            </Typography.Text>
          </Tooltip>
        ) : (
          '-'
        ),
    },
    {
      title: intl.formatMessage({ id: 'setting.video.tasks.created' }),
      dataIndex: 'created_time',
      search: false,
      width: 170,
      render: (_, record) =>
        record.created_time
          ? dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm:ss')
          : '-',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      valueType: 'option',
      width: 130,
      render: (_, record) => (
        <Space>
          {record.status === 2 && record.playlist?.default_url && (
            <Button type="link" size="small" onClick={() => setPreview(record)}>
              {intl.formatMessage({ id: 'setting.video.tasks.preview' })}
            </Button>
          )}
          {record.status === 3 && (
            <Button
              type="link"
              size="small"
              loading={retryingId === record.id}
              disabled={retryingId !== null}
              onClick={() => handleRetry(record.id)}
            >
              {intl.formatMessage({ id: 'setting.video.tasks.retry' })}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  const previewURL = preview?.playlist?.default_url || '';
  return (
    <>
      <Card
        size="small"
        title={intl.formatMessage({ id: 'setting.video.tasks.title' })}
        extra={
          <Space>
            <Button onClick={handleBatch}>
              {intl.formatMessage({ id: 'setting.video.tasks.batch' })}
            </Button>
            <Button onClick={() => actionRef.current?.reload()}>
              {intl.formatMessage({ id: 'setting.video.tasks.refresh' })}
            </Button>
          </Space>
        }
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          request={requestTasks}
          columns={columns}
          search={{ labelWidth: 'auto' }}
          pagination={{ defaultPageSize: 20, showSizeChanger: true }}
          options={false}
          tableAlertOptionRender={false}
          scroll={{ x: 1080 }}
        />
      </Card>
      <Modal
        open={Boolean(preview)}
        title={
          preview?.attachment?.file_name ||
          intl.formatMessage({ id: 'setting.video.tasks.preview' })
        }
        footer={null}
        width={760}
        onCancel={() => setPreview(null)}
        destroyOnClose
      >
        {preview?.playlist?.format === 'hls' && (
          <Alert
            type="info"
            showIcon
            message={intl.formatMessage({
              id: 'setting.video.tasks.preview.hls',
            })}
            style={{ marginBottom: 16 }}
          />
        )}
        <VideoPlayer
          key={previewURL}
          src={previewURL}
          poster={preview?.playlist?.poster_url}
          onError={() => setPlaybackError(true)}
        />
        {playbackError && (
          <Alert
            type="warning"
            showIcon
            message={intl.formatMessage({
              id: 'setting.video.tasks.playback.error',
            })}
            style={{ marginTop: 16 }}
          />
        )}
      </Modal>
    </>
  );
};

export default VideoTasks;
