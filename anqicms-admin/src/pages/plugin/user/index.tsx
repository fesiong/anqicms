import {
  pluginDeleteUserInfo,
  pluginGetUserGroups,
  pluginGetUsers,
} from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProFormRadio,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import UserFieldSetting from './components/setting';
import UserForm from './components/userForm';

const PluginUser: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [currentUser, setCurrentUser] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [balanceVisible, setBalanceVisible] = useState<boolean>(false);
  const [userGroups, setUserGroups] = useState<any[]>([]);
  const [userType, setUserType] = useState<string>('all');
  const intl = useIntl();

  const getUserGroups = () => {
    pluginGetUserGroups().then((res) => {
      setUserGroups(res.data || []);
    });
  };

  useEffect(() => {
    getUserGroups();
  }, []);

  const handleEditUser = async (record: any) => {
    setCurrentUser(record);
    setEditVisible(true);
  };

  const handleChangeBalance = (record: any) => {
    setCurrentUser(record);
    setBalanceVisible(true);
  };

  const handleDelete = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.user.delete.confirm' }),
      onOk: () => {
        pluginDeleteUserInfo(row).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleAddUser = () => {
    setCurrentUser({ status: 1 });
    setEditVisible(true);
  };

  const switchUserType = (type: string) => {
    if (type === userType) {
      return;
    }
    setUserType(type);
    actionRef.current?.reload();
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.retailer.user-id' }),
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.user.user-name' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.user.phone' }),
      dataIndex: 'phone',
    },
    {
      title: intl.formatMessage({ id: 'plugin.user.email' }),
      hideInSearch: true,
      dataIndex: 'email',
      render: (text, record) => {
        return (
          <div>
            {text || record.email}{' '}
            {record.subscribed ? (
              <Tag color="green">
                {intl.formatMessage({ id: 'plugin.user.subscribed' })}
              </Tag>
            ) : (
              <Tag>
                {intl.formatMessage({ id: 'plugin.user.unsubscribed' })}
              </Tag>
            )}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.user.balance' }),
      dataIndex: 'balance',
      render: (text, record) => {
        return (
          <div>
            {record.balance > 0 ? (record.balance / 100).toFixed(2) : '-'}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.user.balance' }),
      dataIndex: 'balance',
      render: (text, record) => {
        return (
          <div>
            {record.balance > 0 ? (record.balance / 100).toFixed(2) : '-'}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.user.avatar_url' }),
      dataIndex: 'full_avatar_url',
      hideInSearch: true,
      width: 70,
      render: (text, record) => {
        return text ? (
          <img src={record.full_avatar_url} className="list-thumb" />
        ) : null;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.user.group' }),
      dataIndex: 'group_id',
      render: (_, entity) => {
        return entity.group?.title;
      },
      renderFormItem: (_, { fieldProps }) => {
        return (
          <ProFormSelect
            name="group_id"
            request={async () => {
              return [
                {
                  title: intl.formatMessage({ id: 'plugin.user.group.all' }),
                  id: 0,
                },
              ].concat(userGroups || []);
            }}
            fieldProps={{
              fieldNames: {
                label: 'title',
                value: 'id',
              },
              ...fieldProps,
            }}
          />
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.retailer.create-time' }),
      dataIndex: 'created_time',
      hideInSearch: true,
      render: (dom: any, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.user.pending' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.user.normal' }),
          status: 'Success',
        },
        [-1]: {
          text: intl.formatMessage({ id: 'plugin.user.blocked' }),
          status: 'Warning',
        },
      },
      renderFormItem: () => {
        return (
          <ProFormRadio.Group
            name="status"
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.user.normal' }),
                value: 'normal',
              },
              {
                label: intl.formatMessage({ id: 'plugin.user.pending' }),
                value: 'pending',
              },
              {
                label: intl.formatMessage({ id: 'plugin.user.blocked' }),
                value: 'blocked',
              },
            ]}
          />
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
            key="edit"
            onClick={() => {
              handleChangeBalance(record);
            }}
          >
            <FormattedMessage id="plugin.user.change-balance" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditUser(record);
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
    <PageContainer>
      <ProTable<any>
        headerTitle={
          <Space>
            <Button
              onClick={() => switchUserType('all')}
              type={userType === 'all' ? 'primary' : 'default'}
            >
              {intl.formatMessage({ id: 'plugin.user.all' })}
            </Button>
            <Button
              onClick={() => switchUserType('subscribed')}
              type={userType === 'subscribed' ? 'primary' : 'default'}
            >
              {intl.formatMessage({ id: 'plugin.user.subscribed' })}
            </Button>
            <Button
              onClick={() => switchUserType('ordered')}
              type={userType === 'ordered' ? 'primary' : 'default'}
            >
              {intl.formatMessage({ id: 'plugin.user.ordered' })}
            </Button>
            <Button
              onClick={() => switchUserType('repurchase')}
              type={userType === 'repurchase' ? 'primary' : 'default'}
            >
              {intl.formatMessage({ id: 'plugin.user.repurchase' })}
            </Button>
          </Space>
        }
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button key="add" onClick={handleAddUser}>
            <FormattedMessage id="plugin.user.add" />
          </Button>,
          <UserFieldSetting key="setting">
            <Button>
              <FormattedMessage id="plugin.user.setting" />
            </Button>
          </UserFieldSetting>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          params.user_type = userType;
          return pluginGetUsers(params);
        }}
        columnsState={{
          persistenceKey: 'user-table',
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
        <UserForm
          open={editVisible}
          user={currentUser}
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
      {balanceVisible && (
        <BalanceForm
          open={balanceVisible}
          user={currentUser}
          onCancel={() => {
            setBalanceVisible(false);
          }}
          onSubmit={async () => {
            setBalanceVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
    </PageContainer>
  );
};

export default PluginUser;
