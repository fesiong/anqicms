import {
  pluginDeleteWechatReplyRule,
  pluginGetWechatReplyRules,
  pluginSaveWechatReplyRule,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';

const PluginWechatReplyRule: React.FC<any> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [currentRule, setCurrentRule] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleDelete = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.wechat.reply.delete.confirm' }),
      onOk: () => {
        pluginDeleteWechatReplyRule(row).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleEdit = (row: any) => {
    setCurrentRule(row);
    setEditVisible(true);
  };

  const handleFinishedEdit = async (values: any) => {
    const data = Object.assign(currentRule, values);
    pluginSaveWechatReplyRule(data)
      .then((res) => {
        message.info(res.msg);
        actionRef.current?.reload();
        setEditVisible(false);
      })
      .catch(() => {
        message.info(intl.formatMessage({ id: 'plugin.wechat.menu.submit.error' }));
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.wechat.reply.keyword' }),
      dataIndex: 'keyword',
    },
    {
      title: intl.formatMessage({ id: 'plugin.wechat.reply.content' }),
      dataIndex: 'content',
    },
    {
      title: intl.formatMessage({ id: 'plugin.wechat.reply.default' }),
      dataIndex: 'is_default',
      valueEnum: {
        1: {
          text: intl.formatMessage({ id: 'plugin.wechat.reply.default.yes' }),
        },
        0: {
          text: '-',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20} key="actions">
          <a
            onClick={() => {
              handleEdit(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            onClick={() => {
              handleDelete(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        width={1000}
        open={visible}
        onCancel={() => {
          setVisible(false);
        }}
        footer={null}
        title={intl.formatMessage({ id: 'plugin.wechat.reply.rule' })}
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          toolBarRender={() => [
            <Button type="primary" key="add" onClick={() => handleEdit({})}>
              <FormattedMessage id="plugin.wechat.reply.rule.add" />
            </Button>,
          ]}
          request={(params) => {
            return pluginGetWechatReplyRules(params);
          }}
          search={false}
          columnsState={{
            persistenceKey: 'wechat-reply-rule-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          rowSelection={false}
          pagination={{
            showSizeChanger: true,
          }}
        />
      </Modal>
      {editVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.wechat.reply.rule.edit' })}
          width={600}
          open={editVisible}
          initialValues={currentRule}
          onOpenChange={(flag) => {
            setEditVisible(flag);
          }}
          onFinish={handleFinishedEdit}
        >
          <ProFormText name="keyword" label={intl.formatMessage({ id: 'plugin.wechat.reply.keyword' })} width="lg" extra={intl.formatMessage({ id: 'plugin.wechat.reply.keyword.description' })} />
          <ProFormTextArea name="content" label={intl.formatMessage({ id: 'plugin.wechat.reply.content' })} width="lg" />
          <ProFormRadio.Group
            name="is_default"
            label={intl.formatMessage({ id: 'plugin.wechat.reply.default' })}
            width="lg"
            extra={intl.formatMessage({ id: 'plugin.wechat.reply.default.description' })}
            valueEnum={{
              0: intl.formatMessage({ id: 'plugin.wechat.reply.default.set-no' }),
              1: intl.formatMessage({ id: 'plugin.wechat.reply.default.set-yes' }),
            }}
          />
        </ModalForm>
      )}
    </>
  );
};

export default PluginWechatReplyRule;
