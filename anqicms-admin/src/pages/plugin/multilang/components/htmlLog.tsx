import { pluginGetMultiLangTranslateHtmlLog } from '@/services';
import { supportLanguages } from '@/utils';
import { ExclamationCircleOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Modal, Tooltip } from 'antd';
import dayjs from 'dayjs';
import React, { useRef } from 'react';

export type HtmlLogProps = {
  open: boolean;
  onCancel: () => void;
};

const TranslateHtmlLog: React.FC<HtmlLogProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({
        id: 'plugin.multilang.html-log.create-time',
      }),
      dataIndex: 'created_time',
      render: (item) => {
        return dayjs((item as number) * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.html-log.uri' }),
      dataIndex: 'uri',
    },
    {
      title: intl.formatMessage({
        id: 'plugin.multilang.html-log.to-language',
      }),
      dataIndex: 'to_language',
      render: (text, record: any) => {
        return (
          supportLanguages.find((item) => item.value === record.to_language)
            ?.label || text
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.html-log.status' }),
      dataIndex: 'status',
      render: (_, record: any) => {
        return record.status === 1 ? (
          <span>
            <FormattedMessage id="plugin.multilang.html-log.status.success" />
          </span>
        ) : (
          <Tooltip title={record.remark}>
            <span className="text-red">
              <FormattedMessage id="plugin.multilang.html-log.status.failure" />
            </span>
            {record.remark && (
              <ExclamationCircleOutlined className="error-icon" />
            )}
          </Tooltip>
        );
      },
    },
  ];

  return (
    <>
      <Modal
        width={1000}
        title={intl.formatMessage({ id: 'plugin.multilang.translate-log' })}
        open={props.open}
        onCancel={props.onCancel}
        footer={null}
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          search={false}
          ghost
          toolBarRender={false}
          request={(params) => {
            return pluginGetMultiLangTranslateHtmlLog(params);
          }}
          columnsState={{
            persistenceKey: 'translate-html-log-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Modal>
    </>
  );
};

export default TranslateHtmlLog;
