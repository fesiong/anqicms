import { pluginGetHtmlCachePushLogs, pluginHtmlCachePush } from '@/services';
import { ExclamationCircleOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, Tooltip } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

export type PushLogProps = {
  storageUrl: string;
  status: string;
  open: boolean;
  onCancel: () => void;
};

const HtmlPushLog: React.FC<PushLogProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [_, setSelectedRowKeys] = useState<any[]>([]);
  const intl = useIntl();

  const handleRePush = async (keys: any[]) => {
    pluginHtmlCachePush({ paths: keys }).then(() => {
      setSelectedRowKeys([]);
      actionRef.current?.reload?.();
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.finance.time' }),
      dataIndex: 'created_time',
      render: (item) => {
        return dayjs((item as number) * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.htmlcache.remote-file' }),
      dataIndex: 'remote_file',
      render: (_, record: any) => {
        return (
          <div className="word-wrap">
            <a
              href={props.storageUrl + '/' + record.remote_file}
              target="_blank"
              rel="noreferrer"
            >
              {record.remote_file}
            </a>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.htmlcache.local-file' }),
      dataIndex: 'local_file',
      render: (_, record: any) => {
        return <div className="word-wrap">{record.local_file}</div>;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.htmlcache.push-status' }),
      dataIndex: 'status',
      render: (_, record: any) => {
        return record.status === 1 ? (
          <span>
            <FormattedMessage id="plugin.htmlcache.push-status.success" />
          </span>
        ) : (
          <Tooltip title={record.error_msg}>
            <span className="text-red">
              <FormattedMessage id="plugin.htmlcache.push-status.failure" />
            </span>
            {record.error_msg && (
              <ExclamationCircleOutlined className="error-icon" />
            )}
          </Tooltip>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <span
            className={record.status === 0 ? 'text-red link' : 'link'}
            key="push"
            onClick={async () => {
              handleRePush([record.local_file]);
            }}
          >
            <FormattedMessage id="plugin.htmlcache.re-push" />
          </span>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Modal
        width={1000}
        title={intl.formatMessage({ id: 'plugin.htmlcache.push-log' })}
        open={props.open}
        onCancel={props.onCancel}
        footer={null}
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="local_file"
          search={false}
          ghost
          toolBarRender={false}
          tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
            <Space>
              <Button
                size={'small'}
                onClick={async () => {
                  handleRePush(selectedRowKeys);
                }}
              >
                <FormattedMessage id="plugin.htmlcache.re-push" />
              </Button>
              <Button type="link" size={'small'} onClick={onCleanSelected}>
                <FormattedMessage id="content.option.cancel-select" />
              </Button>
            </Space>
          )}
          request={(params) => {
            params.status = props.status || '';
            return pluginGetHtmlCachePushLogs(params);
          }}
          columnsState={{
            persistenceKey: 'html-pushlog-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          rowSelection={{
            onChange: (selectedRowKeys) => {
              setSelectedRowKeys(selectedRowKeys);
            },
          }}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Modal>
    </>
  );
};

export default HtmlPushLog;
