import ReplaceKeywords from '@/components/replaceKeywords';
import { getArchives, startCollectorArticle } from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Alert, Button, Card, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import CollectorSetting from './components/setting';

const PluginCollector: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [replaceVisible, setReplaceVisible] = useState<boolean>(false);
  const intl = useIntl();

  const startToCollect = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.collector.start.confirm' }),
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
            actionRef.current?.reloadAndRest?.();
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleEditArchive = async (record: any) => {
    history.push('/archive/detail?id=' + record.id);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
      hideInSearch: true,
      render: (dom, entity) => {
        return (
          <div style={{ maxWidth: 400 }}>
            <a href={entity.link} target="_blank">
              {dom}
            </a>
          </div>
        );
      },
    },
    {
      title: 'thumb',
      dataIndex: 'thumb',
      hideInSearch: true,
      width: 70,
      render: (text, record) => {
        return text ? <img src={record.thumb} className="list-thumb" /> : null;
      },
    },
    {
      title: intl.formatMessage({ id: 'content.module.name' }),
      dataIndex: 'module_name',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.status.draft' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'content.status.normal' }),
          status: 'Success',
        },
        2: {
          text: intl.formatMessage({ id: 'content.status.plan' }),
          status: 'Warning',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'content.create-update-time' }),
      hideInSearch: true,
      dataIndex: 'created_time',
      render: (_, record) => {
        return (
          <div>
            <div>
              {dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm')}
            </div>
            <div>
              {dayjs(record.updated_time * 1000).format('YYYY-MM-DD HH:mm')}
            </div>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="delete"
            onClick={async () => {
              handleEditArchive(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card>
        <Alert
          message={
            <div>
              <FormattedMessage id="plugin.collector.tips" />
            </div>
          }
        />

        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          search={false}
          toolBarRender={() => [
            <CollectorSetting onCancel={() => {}} key="setting">
              <Button>
                <FormattedMessage id="plugin.collector.setting" />
              </Button>
            </CollectorSetting>,
            <Button
              key="keywords"
              onClick={() => {
                startToCollect();
              }}
            >
              <FormattedMessage id="plugin.collector.start" />
            </Button>,
            <Button
              key="keywords"
              onClick={() => {
                history.push('/plugin/contentops/keyword');
              }}
            >
              <FormattedMessage id="menu.plugin.contentops.keyword" />
            </Button>,
            <Button
              key="replace"
              onClick={() => {
                setReplaceVisible(true);
              }}
            >
              <FormattedMessage id="plugin.collector.replace" />
            </Button>,
          ]}
          request={(params) => {
            params.collect = true;
            return getArchives(params);
          }}
          columnsState={{
            persistenceKey: 'archive-collect-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Card>
      <ReplaceKeywords
        open={replaceVisible}
        onCancel={() => {
          setReplaceVisible(false);
        }}
      />
    </PageContainer>
  );
};

export default PluginCollector;
