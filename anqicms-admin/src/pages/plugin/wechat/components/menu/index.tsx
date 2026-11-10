import {
  pluginDeleteWechatMenu,
  pluginGetWechatMenus,
  pluginSaveWechatMenu,
  pluginSyncWechatMenu,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';

const PluginWechatMenu: React.FC<any> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [currentMenu, setCurrentMenu] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleDelete = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.wechat.menu.delete.confirm' }),
      onOk: () => {
        pluginDeleteWechatMenu(row).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleEdit = (row: any) => {
    setCurrentMenu(row);
    setEditVisible(true);
  };

  const handleFinishedEdit = async (values: any) => {
    const data = Object.assign(currentMenu, values);
    pluginSaveWechatMenu(data)
      .then((res) => {
        message.info(res.msg);
        actionRef.current?.reload();
        setEditVisible(false);
      })
      .catch(() => {
        message.info(intl.formatMessage({ id: 'plugin.wechat.menu.submit.error' }));
      });
  };

  const handleSyncMenu = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.wechat.menu.submit.confirm' }),
      content: intl.formatMessage({ id: 'plugin.wechat.menu.submit.content' }),
      onOk: () => {
        pluginSyncWechatMenu({}).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.wechat.menu.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.wechat.menu.type' }),
      dataIndex: 'type',
      valueEnum: {
        click: {
          text: intl.formatMessage({ id: 'plugin.wechat.menu.type.click' }),
        },
        view: {
          text: intl.formatMessage({ id: 'plugin.wechat.menu.type.view' }),
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.wechat.menu.value' }),
      dataIndex: 'value',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20} key="actions">
          <a
            onClick={() => {
              handleDelete(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
          <a
            onClick={() => {
              handleEdit(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
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
        title={intl.formatMessage({ id: 'plugin.wechat.menu' })}
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          headerTitle={
            <div>
              <FormattedMessage id="plugin.wechat.menu.tips" />
            </div>
          }
          toolBarRender={() => [
            <Button key="add" onClick={() => handleSyncMenu()}>
              <FormattedMessage id="plugin.wechat.menu.submit" />
            </Button>,
            <Button type="primary" key="add" onClick={() => handleEdit({})}>
              <FormattedMessage id="plugin.wechat.menu.add" />
            </Button>,
          ]}
          request={(params) => {
            return pluginGetWechatMenus(params);
          }}
          search={false}
          columnsState={{
            persistenceKey: 'wechat-menu-table',
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
          title={intl.formatMessage({ id: 'plugin.wechat.menu.add' })}
          width={600}
          open={editVisible}
          initialValues={currentMenu}
          onOpenChange={(flag) => {
            setEditVisible(flag);
          }}
          onFinish={handleFinishedEdit}
        >
          <ProFormSelect
            name="parent_id"
            request={async () => {
              const res = await pluginGetWechatMenus({});
              return [{ name: intl.formatMessage({ id: 'plugin.wechat.menu.top' }), id: 0 }].concat(
                res.data || [],
              );
            }}
            fieldProps={{
              fieldNames: {
                label: 'name',
                value: 'id',
              },
            }}
          />
          <ProFormSelect
            name="type"
            valueEnum={{
              click: {
                text: intl.formatMessage({ id: 'plugin.wechat.menu.type.click' }),
              },
              view: {
                text: intl.formatMessage({ id: 'plugin.wechat.menu.type.view' }),
              },
            }}
          />
          <ProFormText
            name="name"
            label={intl.formatMessage({ id: 'plugin.wechat.menu.name' })}
            width="lg"
          />
          <ProFormText
            name="value"
            label={intl.formatMessage({ id: 'plugin.wechat.menu.value' })}
            width="lg"
            extra={intl.formatMessage({ id: 'plugin.wechat.menu.value.description' })}
          />
          <ProFormDigit
            name="sort"
            label={intl.formatMessage({ id: 'content.sort.name' })}
            width="lg"
            extra={intl.formatMessage({ id: 'plugin.wechat.sort.description' })}
          />
        </ModalForm>
      )}
    </>
  );
};

export default PluginWechatMenu;
