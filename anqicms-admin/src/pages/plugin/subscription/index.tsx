import NewContainer from '@/components/NewContainer';
import {
  pluginDeleteSubscriber,
  pluginGetSubscriberCategories,
  pluginGetSubscribers,
  pluginSaveSubscriber,
} from '@/services/plugin/subscriber';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import SubscriptionSendMail from './components/sendMail';
import SubscriptionCategory from './components/subscriptionCategory';

const PluginSubscription: React.FC = () => {
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [currentSubscriber, setCurrentSubscriber] = useState<any>({});
  const [categoryVisible, setCategoryVisible] = useState<boolean>(false);
  const [sendVisible, setSendVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  useEffect(() => {}, []);

  const handleEdit = (record: any) => {
    setVisible(true);
    setCurrentSubscriber(record);
  };

  const handleDelete = (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.subscription.delete.confirm' }),
      onOk: async () => {
        if (!selectedRowKeys.length) return true;
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteSubscriber({
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

  const onSaveSubscriber = async (values: any) => {
    const postData = Object.assign(currentSubscriber, values);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveSubscriber(postData)
      .then((res) => {
        message.info(res.msg);
        actionRef.current?.reload();
        setVisible(false);
      })
      .finally(() => {
        hide();
      });
  };

  const handleShowCategories = () => {
    setCategoryVisible(true);
  };

  const handleSendVisible = () => {
    setSendVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.subscription.created-time' }),
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'plugin.subscription.email' }),
      dataIndex: 'email',
    },
    {
      title: intl.formatMessage({ id: 'plugin.subscription.remark' }),
      dataIndex: 'remark',
    },
    {
      title: intl.formatMessage({ id: 'plugin.subscription.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({
            id: 'plugin.subscription.status.inactive',
          }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({
            id: 'plugin.subscription.status.active',
          }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({
        id: 'plugin.subscription.send-times-errors',
      }),
      width: 160,
      dataIndex: 'send_times',
      render: (_: any, record: any) => (
        <span>
          {record.send_times} / {record.error_times}
        </span>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEdit(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a onClick={() => handleDelete([record.id])}>
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
        rowKey="id"
        actionRef={actionRef}
        search={false}
        pagination={false}
        toolBarRender={() => [
          <SubscriptionCategory key="category" onCancel={() => {}}>
            <Button key="category" onClick={() => handleShowCategories()}>
              <FormattedMessage id="plugin.subscription.category" />
            </Button>
          </SubscriptionCategory>,
          <Button key="add" onClick={() => handleEdit({})}>
            <FormattedMessage id="plugin.subscription.add" />
          </Button>,
          <Button key="send" onClick={() => handleSendVisible()}>
            <FormattedMessage id="plugin.subscription.send" />
          </Button>,
        ]}
        request={(params) => {
          return pluginGetSubscribers(params);
        }}
        columnsState={{
          persistenceKey: 'subscribers-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
      />
      {visible && (
        <ModalForm
          width={600}
          title={intl.formatMessage({ id: 'plugin.subscription.edit' })}
          initialValues={currentSubscriber}
          open={visible}
          layout="horizontal"
          onOpenChange={(flag) => {
            if (!flag) {
              setVisible(flag);
            }
          }}
          onFinish={async (values) => {
            onSaveSubscriber(values);
          }}
        >
          <ProFormSelect
            label={intl.formatMessage({
              id: 'plugin.subscription.category.name',
            })}
            name="category_id"
            width="lg"
            request={async () => {
              const res = await pluginGetSubscriberCategories();
              const data = [
                {
                  id: 0,
                  title: intl.formatMessage({
                    id: 'plugin.subscription.category.empty',
                  }),
                },
              ]
                .concat(res.data || [])
                .map((item) => ({ label: item.title, value: item.id }));

              return data;
            }}
          />
          <ProFormText
            name="email"
            width="lg"
            label={intl.formatMessage({ id: 'plugin.subscription.email' })}
          />
          <ProFormRadio.Group
            name="status"
            label={intl.formatMessage({ id: 'plugin.subscription.status' })}
            options={[
              {
                value: 0,
                label: intl.formatMessage({
                  id: 'plugin.subscription.status.inactive',
                }),
              },
              {
                value: 1,
                label: intl.formatMessage({
                  id: 'plugin.subscription.status.active',
                }),
              },
            ]}
          />
          <ProFormText
            name="remark"
            width="lg"
            label={intl.formatMessage({ id: 'plugin.subscription.remark' })}
          />
        </ModalForm>
      )}
      {sendVisible && (
        <SubscriptionSendMail
          open={sendVisible}
          onCancel={() => setSendVisible(false)}
        />
      )}
    </NewContainer>
  );
};

export default PluginSubscription;
