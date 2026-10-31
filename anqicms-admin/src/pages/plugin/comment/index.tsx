import NewContainer from '@/components/NewContainer';
import {
  pluginCheckComment,
  pluginDeleteComment,
  pluginGetComments,
} from '@/services/plugin/comment';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import CommentForm from './components/commentForm';

const PluginComment: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [statusVisible, setStatusVisible] = useState<boolean>(false);
  const [currentComment, setCurrentComment] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.comment.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteComment({
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

  const handleSetStatus = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginCheckComment({
      status: Number(values.status),
      ids: selectedRowKeys,
    })
      .then((res) => {
        message.success(res.msg);
        setStatusVisible(false);
        setSelectedRowKeys([]);
        actionRef.current?.reloadAndRest?.();
      })
      .finally(() => {
        hide();
      });
  };

  const handlePreviewComment = async (record: any) => {
    setCurrentComment(record);
    setEditVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.comment.time' }),
      width: 160,
      dataIndex: 'created_time',
      render: (_, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'plugin.comment.item-title' }),
      width: 200,
      dataIndex: 'item_title',
      render: (text) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.comment.user-name' }),
      width: 100,
      dataIndex: 'user_name',
      render: (text) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.comment.content' }),
      dataIndex: 'content',
      render: (text) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: 'IP',
      dataIndex: 'ip',
      width: 100,
    },
    {
      title: intl.formatMessage({ id: 'plugin.comment.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      width: 100,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.comment.status.default' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.comment.status.ok' }),
          status: 'Success',
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.comment.status.spam' }),
          color: 'red',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      width: 150,
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="check"
            onClick={() => {
              handlePreviewComment(record);
            }}
          >
            <FormattedMessage id="plugin.comment.view-edit" />
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
      <Card key={newKey}>
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          search={false}
          tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
            <Space>
              <Button
                size={'small'}
                onClick={async () => {
                  await setStatusVisible(true);
                }}
              >
                <FormattedMessage id="plugin.comment.batch-update-status" />
              </Button>
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
            return pluginGetComments(params);
          }}
          columnsState={{
            persistenceKey: 'comment-table',
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
          <CommentForm
            open={editVisible}
            editingComment={currentComment}
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
        {statusVisible && (
          <ModalForm
            width={480}
            title={intl.formatMessage({ id: 'plugin.comment.new-status' })}
            open={statusVisible}
            onFinish={handleSetStatus}
            onOpenChange={(e) => setStatusVisible(e)}
          >
            <ProFormRadio.Group
              name="status"
              valueEnum={{
                0: intl.formatMessage({ id: 'content.category.status.hide' }),
                1: intl.formatMessage({ id: 'content.category.status.ok' }),
              }}
            />
          </ModalForm>
        )}
      </Card>
    </NewContainer>
  );
};

export default PluginComment;
