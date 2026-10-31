import {
  pluginGetMultiLangTranslateHtmlCache,
  pluginMultiLangTranslateHtmlCacheRemove,
} from '@/services';
import { supportLanguages } from '@/utils';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space } from 'antd';
import dayjs from 'dayjs';
import React, { useRef } from 'react';

export type HtmlCacheProps = {
  open: boolean;
  onCancel: () => void;
};

const TranslateHtmlCache: React.FC<HtmlCacheProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const handleRemoveCache = (uris: string[]) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.multilang.html-cache.delete-confirm',
      }),
      onOk: () => {
        pluginMultiLangTranslateHtmlCacheRemove({ uris }).then(() => {
          actionRef.current?.reload?.();
        });
      },
    });
  };

  const handleRemoveCacheAll = () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.multilang.html-cache.crean-all-confirm',
      }),
      onOk: () => {
        pluginMultiLangTranslateHtmlCacheRemove({ all: true }).then(() => {
          actionRef.current?.reload?.();
        });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({
        id: 'plugin.multilang.html-cache.create-time',
      }),
      hideInSearch: true,
      dataIndex: 'last_mod',
      render: (item) => {
        return dayjs((item as number) * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.multilang.html-cache.uri' }),
      dataIndex: 'uri',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'plugin.multilang.html-cache.language',
      }),
      hideInSearch: true,
      dataIndex: 'lang',
      render: (text, record: any) => {
        return (
          supportLanguages.find((item) => item.value === record.lang)?.label ||
          text
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      hideInSearch: true,
      render: (_, record) => (
        <Space size={20}>
          <span
            className={record.status === 0 ? 'text-red link' : 'link'}
            key="push"
            onClick={async () => {
              handleRemoveCache([record.uri]);
            }}
          >
            <FormattedMessage id="plugin.multilang.html-cache.delete" />
          </span>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Modal
        width={1000}
        title={intl.formatMessage({ id: 'plugin.multilang.translate-cache' })}
        open={props.open}
        onCancel={props.onCancel}
        footer={null}
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          ghost
          toolBarRender={() => [
            <Button
              key="page"
              type="primary"
              onClick={() => {
                handleRemoveCacheAll();
              }}
            >
              <FormattedMessage id="plugin.multilang.translate-cache.clear-all" />
            </Button>,
          ]}
          request={(params) => {
            return pluginGetMultiLangTranslateHtmlCache(params);
          }}
          columnsState={{
            persistenceKey: 'translate-html-cache-table',
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

export default TranslateHtmlCache;
