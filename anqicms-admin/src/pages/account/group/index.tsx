import {
  deleteAdminGroupInfo,
  getAdminGroups,
  getPermissionMenus,
  saveAdminGroupInfo,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormRadio,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { Alert, Button, Checkbox, Collapse, Modal, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const { Panel } = Collapse;

const AdminGroupList: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const actionRef = useRef<ActionType>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [editInfo, setEditInfo] = useState<any>({});
  const [permissionGroups, setPermissionGroups] = useState<any[]>([]);
  const [permissions, setPermissions] = useState<any[]>([]);
  const intl = useIntl();

  const { currentUser } = initialState;

  const initPermissions = () => {
    getPermissionMenus().then((res) => {
      setPermissionGroups(res.data || []);
    });
  };

  useEffect(() => {
    initPermissions();
  }, []);

  const onPermissionChange = (item: any, e: any) => {
    let checked = e.target.checked;
    if (checked) {
      permissions.push(item.path);
    } else {
      let index = permissions.indexOf(item.path);
      permissions.splice(index, 1);
    }
    setPermissions([].concat(...permissions));
  };

  const handleEdit = (record: any) => {
    setEditInfo(record);
    setPermissions(record.setting?.permissions || []);
    setEditVisible(true);
  };

  const onSubmitEdit = async (data: any) => {
    let values = Object.assign(editInfo, data);
    if (typeof values.setting === 'undefined') {
      values.setting = {};
    }
    values.status = Number(values.status);
    values.setting.permissions = permissions;
    saveAdminGroupInfo(values).then((res) => {
      if (res.code !== 0) {
        message.error(res.msg);
      } else {
        message.info(res.msg);
        actionRef.current?.reload();
        setEditVisible(false);
      }
    });
  };

  const handleRemove = (record: any) => {
    if (currentUser.group_id === record.id || record.id === 1) {
      message.error(intl.formatMessage({ id: 'account.group.cannot-delete' }));
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'account.group.confirm-delete' }),
      onOk: () => {
        deleteAdminGroupInfo({
          id: record.id,
        }).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };
  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'account.group.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'setting.content.notenable' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'setting.content.enable' }),
          status: 'Success',
        },
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
              handleEdit(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          {record.id === 1 && (
            <a
              className="text-red"
              key="delete"
              onClick={async () => {
                await handleRemove(record);
              }}
            >
              <FormattedMessage id="setting.system.delete" />
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'account.group.list' })}
        rowKey="id"
        actionRef={actionRef}
        search={false}
        request={(params) => {
          return getAdminGroups(params);
        }}
        columnsState={{
          persistenceKey: 'account-group-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEdit({});
            }}
          >
            <PlusOutlined /> <FormattedMessage id="account.group.add" />
          </Button>,
        ]}
      />
      {editVisible && (
        <ModalForm
          width={1000}
          title={intl.formatMessage({ id: 'account.group.edit' })}
          open={editVisible}
          initialValues={editInfo}
          layout="horizontal"
          onFinish={onSubmitEdit}
          onOpenChange={(e) => setEditVisible(e)}
        >
          <div className="mb-normal">
            <Alert message={intl.formatMessage({ id: 'account.group.tips' })} />
          </div>
          <ProFormText
            name="title"
            label={intl.formatMessage({ id: 'account.group.title' })}
            width="lg"
          />
          <ProFormText
            name="description"
            label={intl.formatMessage({ id: 'account.group.remark' })}
            width="lg"
          />
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'website.status' })}
            name="status"
            valueEnum={{
              0: intl.formatMessage({ id: 'setting.content.notenable' }),
              1: intl.formatMessage({ id: 'setting.content.enable' }),
            }}
          />
          <ProFormText
            label={intl.formatMessage({ id: 'account.group.permission' })}
          >
            <Collapse defaultActiveKey={'0'}>
              {permissionGroups.map((group: any, index: number) => (
                <Panel header={group.name} key={index}>
                  {group.menus.map((item: any, i: number) => (
                    <Checkbox
                      key={i}
                      onChange={(e: any) => {
                        onPermissionChange(item, e);
                      }}
                      checked={permissions.indexOf(item.path) !== -1}
                    >
                      {item.name}
                    </Checkbox>
                  ))}
                </Panel>
              ))}
            </Collapse>
          </ProFormText>
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default AdminGroupList;
