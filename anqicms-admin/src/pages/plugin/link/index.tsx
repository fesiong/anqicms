import NewContainer from '@/components/NewContainer';
import {
  pluginCheckLink,
  pluginDeleteLink,
  pluginGetLinks,
} from '@/services/plugin/link';
import { PlusOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Image, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import LinkApi from './components/api';
import LinkForm from './components/linkForm';

const PluginLink: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [, setSelectedRowKeys] = useState<any[]>([]);
  const [currentLink, setCurrentLink] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.link.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (const item of selectedRowKeys) {
            await pluginDeleteLink({
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

  const handleEditLink = async (record: any) => {
    setCurrentLink(record);
    setEditVisible(true);
  };

  const handleCheckLink = async (record: any) => {
    const res = await pluginCheckLink(record);
    message.info(res.msg);
    if (actionRef.current) {
      actionRef.current.reload();
    }
  };

  const getStatusText = (status: any) => {
    if (status === 0) {
      return intl.formatMessage({ id: 'plugin.link.status.wait' });
    } else if (status === 1) {
      return intl.formatMessage({ id: 'plugin.link.status.ok' });
    } else if (status === 2) {
      return 'NOFOLLOW';
    } else if (status === 3) {
      return intl.formatMessage({ id: 'plugin.link.status.wrong-keyword' });
    } else if (status === 4) {
      return intl.formatMessage({ id: 'plugin.link.status.no-back-url' });
    }

    return status;
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.link.id' }),
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.link.other-title-link' }),
      dataIndex: 'title',
      render: (_, record) => {
        return (
          <div>
            <span>{record.title}</span>
            <span> / </span>
            <a href={record.link} target="_blank" rel="noreferrer">
              {record.link}
            </a>
            {record.logo && <Image width={100} src={record.logo} />}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.link.other-contact-remark' }),
      dataIndex: 'contact',
      render: (_, record) => {
        return (
          <div>
            <span>{record.contact}</span>
            <span> / </span>
            <span>{record.remark}</span>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.link.status-check-time' }),
      dataIndex: 'status',
      render: (text, record) => {
        return (
          <div>
            <span>{getStatusText(text)}</span>
            <span> / </span>
            <span>
              {dayjs(record.checked_time * 1000).format('YYYY-MM-DD HH:mm')}
            </span>
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.link.create-time' }),
      dataIndex: 'created_time',
      render: (_, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
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
              handleCheckLink(record);
            }}
          >
            <FormattedMessage id="plugin.link.check" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditLink(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
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
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProTable<any>
        key={newKey}
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <LinkApi key="api">
            <Button>
              <FormattedMessage id="plugin.link.api.title" />
            </Button>
          </LinkApi>,
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditLink({});
            }}
          >
            <PlusOutlined /> <FormattedMessage id="plugin.link.add" />
          </Button>,
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
          return pluginGetLinks(params);
        }}
        columnsState={{
          persistenceKey: 'friendlink-table',
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
        }}
      />
      {editVisible && (
        <LinkForm
          open={editVisible}
          editingLink={currentLink}
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

export default PluginLink;
