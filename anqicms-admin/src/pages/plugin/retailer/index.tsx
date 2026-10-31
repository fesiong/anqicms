import {
  pluginGetRetailers,
  pluginSaveRetailer,
} from '@/services/plugin/retailer';
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
import RetailerSetting from './components/setting';

const PluginRetailer: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentUser, setCurrentUser] = useState<any>({});
  const [realnameVisible, setRealnameVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleSetRealname = (record: any) => {
    setCurrentUser(record);
    setRealnameVisible(true);
  };

  const setRealname = async (values: any) => {
    values.id = currentUser.id;
    pluginSaveRetailer(values).then(() => {
      actionRef.current?.reload();
    });
  };

  const setUserRetailer = async (values: any) => {
    if (!values.cancel) {
      values.is_retailer = 1;
    }
    values.id = Number(values.id);
    pluginSaveRetailer(values).then((res) => {
      setEditVisible(false);
      message.info(res.msg);
      actionRef.current?.reload();
    });
  };

  const handleCancelRetailer = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.retailer.cancel.confirm' }),
      content: intl.formatMessage({ id: 'plugin.retailer.cancel.content' }),
      onOk: () => {
        setUserRetailer({
          id: record.id,
          is_retailer: 0,
          cancel: true,
        });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.retailer.user-id' }),
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.retailer.user-name' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.retailer.real-name' }),
      dataIndex: 'real_name',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.retailer.balance' }),
      dataIndex: 'balance',
      hideInSearch: true,
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.retailer.total-reward' }),
      dataIndex: 'total_reward',
      hideInSearch: true,
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.retailer.create-time' }),
      dataIndex: 'created_time',
      hideInSearch: true,
      render: (_, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
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
              handleSetRealname(record);
            }}
          >
            <FormattedMessage id="plugin.retailer.change-name" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleCancelRetailer(record);
            }}
          >
            <FormattedMessage id="plugin.retailer.cancel" />
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
              setEditVisible(true);
            }}
          >
            <FormattedMessage id="plugin.retailer.add" />
          </Button>,
          <RetailerSetting key="group" onCancel={() => {}}>
            <Button type="primary">
              <FormattedMessage id="plugin.retailer.setting" />
            </Button>
          </RetailerSetting>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetRetailers(params);
        }}
        columnsState={{
          persistenceKey: 'retailer-table',
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
          title={intl.formatMessage({ id: 'plugin.retailer.add.name' })}
          open={editVisible}
          onOpenChange={(flag) => {
            setEditVisible(flag);
          }}
          onFinish={setUserRetailer}
        >
          <ProFormText
            name="id"
            label={intl.formatMessage({ id: 'plugin.retailer.user-id' })}
          />
        </ModalForm>
      )}
      {realnameVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.retailer.change-name' })}
          open={realnameVisible}
          onOpenChange={(flag) => {
            setRealnameVisible(flag);
          }}
          onFinish={setRealname}
        >
          <ProFormText
            name="user_name"
            initialValue={currentUser.user_name}
            label={intl.formatMessage({ id: 'plugin.retailer.user-name' })}
            readonly
          />
          <ProFormText
            name="real_name"
            label={intl.formatMessage({
              id: 'plugin.retailer.change-name.new',
            })}
          />
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default PluginRetailer;
