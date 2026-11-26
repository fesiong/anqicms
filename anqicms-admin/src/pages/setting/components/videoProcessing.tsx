import {
  ProFormCheckbox,
  ProFormDependency,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Alert, Card, Divider, Space } from 'antd';
import React from 'react';
import VideoFfmpegPanel from './videoFfmpeg';
import VideoTasks from './videoTasks';

const VideoProcessing: React.FC = () => {
  const intl = useIntl();
  const yesNo = [
    {
      value: 0,
      label: intl.formatMessage({ id: 'setting.content.notenable' }),
    },
    {
      value: 1,
      label: intl.formatMessage({ id: 'setting.content.enable' }),
    },
  ];

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message={intl.formatMessage({ id: 'setting.video.description' })}
      />
      <Card
        size="small"
        title={intl.formatMessage({ id: 'setting.video.title' })}
      >
        <ProFormRadio.Group
          name={['video', 'enabled']}
          label={intl.formatMessage({ id: 'setting.video.enabled' })}
          options={yesNo}
        />
        <ProFormRadio.Group
          name={['video', 'format']}
          label={intl.formatMessage({ id: 'setting.video.format' })}
          options={[
            {
              value: 'mp4',
              label: intl.formatMessage({ id: 'setting.video.format.mp4' }),
            },
            {
              value: 'hls',
              label: intl.formatMessage({ id: 'setting.video.format.hls' }),
            },
          ]}
        />
        <ProFormCheckbox.Group
          name={['video', 'qualities']}
          label={intl.formatMessage({ id: 'setting.video.qualities' })}
          options={[
            {
              value: 'original',
              label: intl.formatMessage({
                id: 'setting.video.qualities.original',
              }),
            },
            {
              value: '480p',
              label: intl.formatMessage({ id: 'setting.video.qualities.480' }),
            },
            {
              value: '720p',
              label: intl.formatMessage({ id: 'setting.video.qualities.720' }),
            },
            {
              value: '1080p',
              label: intl.formatMessage({ id: 'setting.video.qualities.1080' }),
            },
          ]}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'setting.video.qualities.required',
              }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.video.qualities.original',
          })}
        />
        <ProFormDependency name={['video']}>
          {({ video }) => (
            <>
              <Divider orientation="left" orientationMargin={0}>
                {intl.formatMessage({ id: 'setting.video.poster' })}
              </Divider>
              <ProFormRadio.Group
                name={['video', 'poster_enabled']}
                label={intl.formatMessage({ id: 'setting.video.poster' })}
                options={yesNo}
              />
              {video?.poster_enabled !== 0 && (
                <Space size={16} wrap align="start">
                  <ProFormDigit
                    name={['video', 'poster_time']}
                    label={intl.formatMessage({
                      id: 'setting.video.poster.time',
                    })}
                    min={0}
                    max={86400}
                    width="sm"
                    fieldProps={{ precision: 0 }}
                  />
                  <ProFormDigit
                    name={['video', 'poster_width']}
                    label={intl.formatMessage({
                      id: 'setting.video.poster.width',
                    })}
                    min={160}
                    max={3840}
                    width="sm"
                    fieldProps={{ precision: 0 }}
                  />
                  <ProFormDigit
                    name={['video', 'poster_quality']}
                    label={intl.formatMessage({
                      id: 'setting.video.poster.quality',
                    })}
                    min={1}
                    max={100}
                    width="sm"
                    fieldProps={{ precision: 0, suffix: '%' }}
                  />
                </Space>
              )}
              <Space size={16} wrap align="start">
                <ProFormText
                  name={['video', 'video_codec']}
                  label={intl.formatMessage({
                    id: 'setting.video.codec.video',
                  })}
                  placeholder="libx264"
                  width="md"
                  fieldProps={{ maxLength: 64 }}
                />
                <ProFormText
                  name={['video', 'audio_codec']}
                  label={intl.formatMessage({
                    id: 'setting.video.codec.audio',
                  })}
                  placeholder="aac"
                  width="md"
                  fieldProps={{ maxLength: 64 }}
                />
              </Space>
              <div className="text-muted mb-normal">
                {intl.formatMessage({ id: 'setting.video.codec.tip' })}
              </div>
              {video?.format === 'hls' && (
                <ProFormDigit
                  name={['video', 'hls_segment']}
                  label={intl.formatMessage({
                    id: 'setting.video.hls.segment',
                  })}
                  min={2}
                  max={60}
                  width="sm"
                  fieldProps={{ precision: 0 }}
                />
              )}
              <Divider />
              <ProFormDigit
                name={['video', 'concurrency']}
                label={intl.formatMessage({
                  id: 'setting.video.concurrency',
                })}
                min={1}
                max={4}
                width="sm"
                fieldProps={{ precision: 0 }}
                extra={intl.formatMessage({
                  id: 'setting.video.concurrency.tip',
                })}
              />
              <ProFormDigit
                name={['video', 'max_retries']}
                label={intl.formatMessage({ id: 'setting.video.retries' })}
                min={0}
                max={10}
                width="sm"
                fieldProps={{ precision: 0 }}
              />
              <ProFormDigit
                name={['video', 'timeout']}
                label={intl.formatMessage({ id: 'setting.video.timeout' })}
                min={60}
                max={86400}
                width="sm"
                fieldProps={{ precision: 0 }}
              />
            </>
          )}
        </ProFormDependency>
      </Card>
      <VideoFfmpegPanel />
      <VideoTasks />
    </Space>
  );
};

export default VideoProcessing;
