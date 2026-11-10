import { pluginDeleteWechatMessage, pluginGetWechatMessages } from '@/services';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import WechatMenu from './components/menu';
import WechatReplyRule from './components/replyrule';
import WechatSetting from './components/setting';

const PluginWechatMessage: React.FC<any> = () => {
  const actionRef = useRef<ActionType>();
  const [currentMessage, setCurrentMessage] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleDelete = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.backup.delete.confirm' }),
      onOk: () => {
        pluginDeleteWechatMessage(row).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleReply = (row: any) => {
    setCurrentMessage(row);
    setEditVisible(true);
  };

  const handleFinishedReply = async (values: any) => {
    if (values.reply && values.reply !== currentMessage.reply) {
      pluginDeleteWechatMessage(values)
        .then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        })
        .catch(() => {
          message.info(
            intl.formatMessage({ id: 'plugin.wechat.menu.submit.error' }),
          );
        });
    } else {
      setEditVisible(false);
      if (actionRef.current) {
        actionRef.current.reload();
      }
    }
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.aigenerate.time' }),
      dataIndex: 'created_time',
      width: 150,
      render: (_, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: 'openid',
      dataIndex: 'openid',
      width: 270,
    },
    {
      title: intl.formatMessage({ id: 'plugin.guestbook.content' }),
      dataIndex: 'content',
    },
    {
      title: intl.formatMessage({ id: 'plugin.wechat.reply.content' }),
      dataIndex: 'reply',
    },
    {
      title: intl.formatMessage({ id: 'plugin.wechat.reply.time' }),
      dataIndex: 'reply_time',
      width: 150,
      render: (_, entity) => {
        return entity.reply_time > 0
          ? dayjs(entity.reply_time * 1000).format('YYYY-MM-DD HH:mm')
          : '-';
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 150,
      render: (_, record) => (
        <Space size={20} key="actions">
          <a
            onClick={() => {
              handleDelete(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
          {record.created_time * 1000 > new Date().valueOf() && (
            <a
              onClick={() => {
                handleReply(record);
              }}
            >
              <FormattedMessage id="plugin.wechat.reply" />
            </a>
          )}
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
          <WechatReplyRule key="rule">
            <Button>
              <FormattedMessage id="plugin.wechat.auto-reply.setting" />
            </Button>
          </WechatReplyRule>,
          <WechatMenu key="menu">
            <Button>
              <FormattedMessage id="plugin.wechat.menu.setting" />
            </Button>
          </WechatMenu>,
          <WechatSetting key="setting">
            <Button>
              <FormattedMessage id="plugin.wechat.official.setting" />
            </Button>
          </WechatSetting>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetWechatMessages(params);
        }}
        search={false}
        columnsState={{
          persistenceKey: 'wechat-message-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
      {editVisible && (
        <ModalForm
          open={editVisible}
          initialValues={currentMessage}
          onValuesChange={(flag) => {
            setEditVisible(flag);
          }}
          onFinish={handleFinishedReply}
        >
          <ProFormText name="id" label="ID" width="lg" readonly />
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.aigenerate.time' })}
            width="lg"
            initialValue={dayjs(currentMessage.created_time * 1000).format(
              'YYYY-MM-DD HH:mm',
            )}
            readonly
          />
          <ProFormText name="openid" label="OPENID" width="lg" readonly />
          <ProFormText
            name="content"
            label={intl.formatMessage({ id: 'plugin.guestbook.content' })}
            width="lg"
            readonly
          />
          <ProFormText
            name="reply"
            label={intl.formatMessage({ id: 'plugin.wechat.reply.content' })}
            width="lg"
            extra={intl.formatMessage({
              id: 'plugin.wechat.reply.content.description',
            })}
          />
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default PluginWechatMessage;
