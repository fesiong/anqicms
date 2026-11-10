import { useVipModal } from '@/components/vipModal';
import {
  collectAiGenerateArticle,
  collectCollectorArticle,
  digCollectorKeyword,
} from '@/services';
import {
  pluginDeleteKeyword,
  pluginExportKeyword,
  pluginGetKeywords,
} from '@/services/plugin/keyword';
import { exportFile } from '@/utils';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, Link, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';
import KeywordImport from './components/import';
import KeywordForm from './components/keywordForm';
import KeywordSetting from './components/setting';

const PluginKeyword: React.FC = () => {
  const { checkVip, VipModal } = useVipModal();
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentKeyword, setCurrentKeyword] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.keyword.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          await pluginDeleteKeyword({
            ids: selectedRowKeys,
          });
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          if (actionRef.current) {
            actionRef.current.reloadAndRest?.();
          }
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleEditKeyword = async (record: any) => {
    setCurrentKeyword(record);
    setEditVisible(true);
  };

  const handleExportKeyword = async () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.keyword.export.confirm' }),
      onOk: async () => {
        let res = await pluginExportKeyword();
        exportFile(res.data?.header, res.data?.content, 'csv');
      },
    });
  };

  const handleDigKeyword = async () => {
    checkVip(async () => {
      let res = await digCollectorKeyword();
      message.info(res.msg);
    });
  };

  const handleCollectArticle = (keyword: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.keyword.collect.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'plugin.keyword.collect.doing' }),
          0,
        );
        collectCollectorArticle(keyword)
          .then((res) => {
            if (res.code !== 0) {
              message.error(res.msg);
            } else {
              if (actionRef.current) {
                actionRef.current.reload();
              }
              message.info(res.msg);
            }
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleAiGenerateArticle = (keyword: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.keyword.aigenerate.confirm' }),
      content: intl.formatMessage({ id: 'plugin.keyword.aigenerate.content' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'plugin.keyword.aigenerate.doing' }),
          0,
        );
        collectAiGenerateArticle(keyword)
          .then((res) => {
            if (res.code !== 0) {
              message.error(res.msg);
            } else {
              if (actionRef.current) {
                actionRef.current.reload();
              }
              message.info(res.msg);
            }
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const cleanupKeywords = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.keyword.cleanup.confirm' }),
      content: intl.formatMessage({ id: 'plugin.keyword.cleanup.content' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        await pluginDeleteKeyword({
          all: true,
        });
        hide();
        message.success(intl.formatMessage({ id: 'content.delete.success' }));
        actionRef.current?.reloadAndRest?.();
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      hideInSearch: true,
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.keyword.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.keyword.level' }),
      hideInSearch: true,
      dataIndex: 'level',
    },
    {
      title: intl.formatMessage({ id: 'plugin.keyword.archive-category-id' }),
      hideInSearch: true,
      dataIndex: 'category_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.keyword.article-count' }),
      hideInSearch: true,
      dataIndex: 'article_count',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="collect"
            onClick={() => {
              handleCollectArticle(record);
            }}
          >
            <FormattedMessage id="plugin.keyword.collect" />
          </a>
          <a
            key="collect"
            onClick={() => {
              handleAiGenerateArticle(record);
            }}
          >
            <FormattedMessage id="plugin.keyword.aigenerate" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditKeyword(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <Link key="view" to={'/plugin/contentops/aigenerate?keyword=' + record.title}>
            <FormattedMessage id="plugin.keyword.aigenerate.view-archive" />
          </Link>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              handleRemove([record.id]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditKeyword({});
            }}
          >
            <PlusOutlined /> <FormattedMessage id="plugin.keyword.add" />
          </Button>,
          <Button
            key="export"
            onClick={() => {
              handleExportKeyword();
            }}
          >
            <FormattedMessage id="plugin.keyword.export" />
          </Button>,
          <KeywordImport
            key="import"
            onCancel={() => {
              actionRef.current?.reloadAndRest?.();
            }}
          >
            <Button
              onClick={() => {
                //todo
              }}
            >
              <FormattedMessage id="plugin.keyword.import" />
            </Button>
          </KeywordImport>,
          <Button
            key="update"
            onClick={() => {
              handleDigKeyword();
            }}
          >
            <FormattedMessage id="plugin.keyword.manual-dig" />
          </Button>,
          <KeywordSetting onCancel={() => {}} key="setting">
            <Button>
              <FormattedMessage id="plugin.keyword.dig-setting" />
            </Button>
          </KeywordSetting>,
        ]}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
            <Button
              size={'small'}
              onClick={() => {
                handleRemove(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-delete" />
            </Button>
            <Button size={'small'} onClick={cleanupKeywords}>
              <FormattedMessage id="plugin.keyword.cleanup" />
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              <FormattedMessage id="content.option.cancel-select" />
            </Button>
          </Space>
        )}
        request={(params) => {
          return pluginGetKeywords(params);
        }}
        columnsState={{
          persistenceKey: 'plugin-keywords-table',
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
      {editVisible && (
        <KeywordForm
          open={editVisible}
          editingKeyword={currentKeyword}
          onCancel={() => {
            setEditVisible(false);
          }}
          onSubmit={async () => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
      <VipModal />
    </PageContainer>
  );
};

export default PluginKeyword;
