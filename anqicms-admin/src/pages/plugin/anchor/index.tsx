import NewContainer from '@/components/NewContainer';
import {
  pluginDeleteAnchor,
  pluginExportAnchor,
  pluginGetAnchors,
  pluginReplaceAnchor,
} from '@/services/plugin/anchor';
import { exportFile } from '@/utils';
import { PlusOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';
import AnchorForm from './components/anchorForm';
import AnchorImport from './components/import';
import AnchorSetting from './components/setting';
import './index.less';

const PluginAnchor: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentAnchor, setCurrentAnchor] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.anchor.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteAnchor({
              id: item,
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleEditAnchor = async (record: any) => {
    setCurrentAnchor(record);
    setEditVisible(true);
  };

  const handleReplaceAnchor = async (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.anchor.replace.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );

        let res = await pluginReplaceAnchor(record);
        message.info(res.msg);
        if (actionRef.current) {
          actionRef.current.reload();
        }
        hide();
      },
    });
  };

  const handleExportAnchor = async () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.anchor.export.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );

        let res = await pluginExportAnchor();

        exportFile(res.data?.header, res.data?.content, 'csv');
        hide();
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
      title: intl.formatMessage({ id: 'plugin.anchor.title' }),
      dataIndex: 'title',
      fieldProps() {
        return {
          placeholder: intl.formatMessage({
            id: 'plugin.anchor.title.placeholder',
          }),
        };
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.anchor.link' }),
      hideInSearch: true,
      dataIndex: 'link',
    },
    {
      title: intl.formatMessage({ id: 'plugin.anchor.weight' }),
      hideInSearch: true,
      dataIndex: 'weight',
    },
    {
      title: intl.formatMessage({ id: 'plugin.anchor.replace-count' }),
      hideInSearch: true,
      dataIndex: 'replace_count',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="check"
            onClick={() => {
              handleReplaceAnchor(record);
            }}
          >
            <FormattedMessage id="plugin.anchor.replace" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditAnchor(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
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
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProTable<any>
        key={newKey}
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditAnchor({});
            }}
          >
            <PlusOutlined /> <FormattedMessage id="plugin.anchor.new" />
          </Button>,
          <Button
            key="export"
            onClick={() => {
              handleExportAnchor();
            }}
          >
            <FormattedMessage id="plugin.anchor.export" />
          </Button>,
          <AnchorImport
            key="import"
            onCancel={() => {
              actionRef.current?.reloadAndRest?.();
            }}
          >
            <Button
              key="import"
              onClick={() => {
                //todo
              }}
            >
              <FormattedMessage id="plugin.anchor.import" />
            </Button>
          </AnchorImport>,
          <Button
            key="update"
            onClick={() => {
              handleReplaceAnchor({});
            }}
          >
            <FormattedMessage id="plugin.anchor.batch-update" />
          </Button>,
          <AnchorSetting key="setting">
            <Button
              key="setting"
              onClick={() => {
                //todo
              }}
            >
              <FormattedMessage id="plugin.anchor.setting" />
            </Button>
          </AnchorSetting>,
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
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              <FormattedMessage id="content.option.cancel-select" />
            </Button>
          </Space>
        )}
        request={(params) => {
          return pluginGetAnchors(params);
        }}
        columnsState={{
          persistenceKey: 'anchor-table',
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
        <AnchorForm
          open={editVisible}
          editingAnchor={currentAnchor}
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
    </NewContainer>
  );
};

export default PluginAnchor;
