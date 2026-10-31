import { getAiArticlePlans, startCollectorArticle } from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Card, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef } from 'react';
import CollectorSetting from './components/setting';

const PluginAiGenerate: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const startToCollect = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.aigenerate.start.confirm' }),
      content: intl.formatMessage({
        id: 'plugin.aigenerate.start.description',
      }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        startCollectorArticle()
          .then((res) => {
            message.success(res.msg);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.aigenerate.type' }),
      dataIndex: 'type',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.type.undefine' }),
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.type.generate' }),
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.type.translate' }),
        },
        3: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.type.pseudo' }),
        },
        4: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.type.media' }),
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'content.keywords.name' }),
      dataIndex: 'keyword',
    },
    {
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.aigenerate.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.waiting' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.doing' }),
          status: 'Default',
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.finish' }),
          status: 'Success',
        },
        4: {
          text: intl.formatMessage({ id: 'plugin.aigenerate.error' }),
          status: 'Default',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.aigenerate.time' }),
      hideInSearch: true,
      dataIndex: 'created_time',
      render: (item) => {
        if (`${item}` === '0') {
          return false;
        }
        return dayjs((item as number) * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
  ];

  return (
    <PageContainer>
      <Card>
        <div className="control">
          <Space className="space-wrap" size={20} style={{ width: '100%' }}>
            <CollectorSetting onCancel={() => {}} key="setting">
              <Button>
                <FormattedMessage id="plugin.aigenerate.setting" />
              </Button>
            </CollectorSetting>
            <Button
              key="keywords"
              onClick={() => {
                startToCollect();
              }}
            >
              <FormattedMessage id="plugin.aigenerate.start" />
            </Button>
            <Button
              key="keywords"
              onClick={() => {
                history.push('/plugin/contentops/keyword');
              }}
            >
              <FormattedMessage id="menu.plugin.contentops.keyword" />
            </Button>
          </Space>
        </div>
        <div>
          <ProTable<any>
            actionRef={actionRef}
            rowKey="id"
            request={(params) => {
              return getAiArticlePlans(params);
            }}
            columnsState={{
              persistenceKey: 'ai-article-table',
              persistenceType: 'localStorage',
            }}
            form={{
              initialValues: {
                keyword:
                  new URLSearchParams(window.location.search).get('keyword') ||
                  '',
              },
            }}
            columns={columns}
            pagination={{
              showSizeChanger: true,
              showQuickJumper: true,
            }}
          />
        </div>
        <div className="mt-normal">
          <p>
            <FormattedMessage id="plugin.aigenerate.tips1" />
          </p>
          <p>
            <FormattedMessage id="plugin.aigenerate.tips2" />
          </p>
          <p>
            <FormattedMessage id="plugin.aigenerate.tips3" />
          </p>
          <p>
            <FormattedMessage id="plugin.aigenerate.tips4" />
          </p>
        </div>
      </Card>
    </PageContainer>
  );
};

export default PluginAiGenerate;
