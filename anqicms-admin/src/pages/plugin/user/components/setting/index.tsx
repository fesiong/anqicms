import {
  pluginDeleteUserField,
  pluginGetUserFieldsSetting,
  pluginGetUserGroups,
  pluginSaveUserFieldsSetting,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

export type UserFieldSettingProps = {
  children: React.ReactNode;
};

const UserFieldSetting: React.FC<UserFieldSettingProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [setting, setSetting] = useState<any>({ fields: [] });
  const intl = useIntl();
  const [loading, setLoading] = useState<boolean>(false);

  const getSetting = async () => {
    const res = await pluginGetUserFieldsSetting();
    setSetting(res.data || { fields: [] });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleRemoveItem = (record: any, index: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.guestbook.field.delete.confirm',
      }),
      content: intl.formatMessage({
        id: 'plugin.guestbook.field.delete.confirm.content',
      }),
      onOk: async () => {
        pluginDeleteUserField({ field_name: record.field_name });
        setting.fields.splice(index, 1);
        setting.fields = [].concat(setting.fields);
        setSetting(setting);
        if (actionRef.current) {
          actionRef.current.reload();
        }
      },
    });
  };

  const handleSaveField = async (values: any) => {
    let exists = false;
    if (!setting.fields) {
      setting.fields = [];
    }
    if (loading) {
      return;
    }
    setLoading(true);
    const hide = message.loading('loading', 0);
    for (const i in setting.fields) {
      if (setting.fields[i].field_name === values.field_name) {
        exists = true;
        setting.fields[i] = Object.assign(setting.fields[i], values);
      }
    }
    if (!exists) {
      setting.fields.push(values);
    }
    setting.fields = [].concat(setting.fields);
    setSetting(setting);
    hide();
    setLoading(false);
    if (actionRef.current) {
      actionRef.current.reload();
    }
    setEditVisible(false);
  };

  const handleSaveSetting = async () => {
    if (loading) {
      return;
    }
    setLoading(true);
    const hide = message.loading('loading', 0);

    pluginSaveUserFieldsSetting(setting)
      .then((res) => {
        if (res.code === 0) {
          message.success(res.msg);
          setEditVisible(false);
          if (actionRef.current) {
            actionRef.current.reload();
          }
        } else {
          message.error(res.msg);
        }
      })
      .finally(() => {
        setLoading(false);
        hide();
      });
  };

  const handleSetDefaultStatus = (e: any) => {
    setting.default_status = e.target.value;
    setSetting(setting);
  };

  const handleSetDefaultGroupId = (e: any) => {
    setting.default_group_id = e;
    setSetting(setting);
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'content.module.field.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'content.module.field.field-name' }),
      dataIndex: 'field_name',
    },
    {
      title: intl.formatMessage({ id: 'content.module.field.type' }),
      dataIndex: 'type',
      render: (text: any, record) => (
        <div>
          <span>
            {record.is_system
              ? intl.formatMessage({ id: 'content.module.field.type.built-in' })
              : ''}
          </span>
          <span>{text}</span>
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      render: (_, record, index) => (
        <Space size={20}>
          <>
            <a
              onClick={() => {
                setCurrentField(record);
                setEditVisible(true);
              }}
            >
              <FormattedMessage id="setting.action.edit" />
            </a>
            {!record.is_system && (
              <a
                className="text-red"
                onClick={() => {
                  handleRemoveItem(record, index);
                }}
              >
                <FormattedMessage id="setting.system.delete" />
              </a>
            )}
          </>
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
        width={800}
        title={intl.formatMessage({ id: 'plugin.user.setting' })}
        open={visible}
        onCancel={() => {
          setVisible(false);
        }}
        onOk={() => {
          handleSaveSetting();
        }}
      >
        <ProFormSelect
          label={intl.formatMessage({ id: 'plugin.user.default_group_id' })}
          name="group_id"
          request={async () => {
            const res = await pluginGetUserGroups();
            return res.data || [];
          }}
          fieldProps={{
            fieldNames: {
              label: 'title',
              value: 'id',
            },
            defaultValue: setting.default_group_id,
          }}
          extra={intl.formatMessage({
            id: 'plugin.user.default_group_id.description',
          })}
          onChange={handleSetDefaultGroupId}
        />
        <ProFormRadio.Group
          label={intl.formatMessage({ id: 'plugin.user.default_status' })}
          name="default_status"
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
          fieldProps={{
            onChange: handleSetDefaultStatus,
            defaultValue: setting.default_status,
          }}
          extra={intl.formatMessage({
            id: 'plugin.user.default_status.description',
          })}
        />
        <ProTable<any>
          rowKey="name"
          search={false}
          actionRef={actionRef}
          toolBarRender={() => [
            <Button
              key="add"
              type="primary"
              onClick={() => {
                setCurrentField({ type: 'text', required: false });
                setEditVisible(true);
              }}
            >
              <FormattedMessage id="content.module.field.add" />
            </Button>,
          ]}
          tableAlertRender={false}
          tableAlertOptionRender={false}
          request={async () => {
            return {
              data: setting.fields || [],
              success: true,
            };
          }}
          columnsState={{
            persistenceKey: 'user-fields-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          pagination={false}
        />
      </Modal>
      {editVisible && (
        <ModalForm
          width={600}
          title={
            currentField.name
              ? currentField.name +
                intl.formatMessage({ id: 'content.module.field.edit' })
              : intl.formatMessage({ id: 'content.module.field.add' })
          }
          open={editVisible}
          modalProps={{
            onCancel: () => {
              setEditVisible(false);
            },
          }}
          initialValues={currentField}
          layout="horizontal"
          onFinish={async (values) => {
            handleSaveField(values);
          }}
        >
          <ProFormText
            name="name"
            required
            label={intl.formatMessage({ id: 'content.module.field.name' })}
            extra={intl.formatMessage({
              id: 'plugin.user.setting.name.description',
            })}
          />
          <ProFormText
            name="field_name"
            label={intl.formatMessage({
              id: 'content.module.field.field-name',
            })}
            disabled={currentField.field_name ? true : false}
            extra={intl.formatMessage({
              id: 'content.module.field.field-name.description',
            })}
          />
          <ProFormRadio.Group
            name="type"
            label={intl.formatMessage({ id: 'content.module.field.type' })}
            disabled={currentField.field_name ? true : false}
            valueEnum={{
              text: intl.formatMessage({
                id: 'content.module.field.type.text',
              }),
              number: intl.formatMessage({
                id: 'content.module.field.type.number',
              }),
              textarea: intl.formatMessage({
                id: 'content.module.field.type.textarea',
              }),
              editor: intl.formatMessage({
                id: 'content.module.field.type.editor',
              }),
              radio: intl.formatMessage({
                id: 'content.module.field.type.radio',
              }),
              checkbox: intl.formatMessage({
                id: 'content.module.field.type.checkbox',
              }),
              select: intl.formatMessage({
                id: 'content.module.field.type.select',
              }),
              image: intl.formatMessage({
                id: 'content.module.field.type.image',
              }),
              images: intl.formatMessage({
                id: 'content.module.field.type.images',
              }),
              file: intl.formatMessage({
                id: 'content.module.field.type.file',
              }),
              texts: intl.formatMessage({
                id: 'content.module.field.type.texts',
              }),
              archive: intl.formatMessage({
                id: 'content.module.field.type.archive',
              }),
              category: intl.formatMessage({
                id: 'content.module.field.type.category',
              }),
            }}
          />
          <ProFormTextArea
            label={intl.formatMessage({ id: 'content.param.default' })}
            name="content"
            fieldProps={{
              rows: 4,
            }}
            extra={intl.formatMessage({
              id: 'content.module.field.default.description',
            })}
          />
        </ModalForm>
      )}
    </>
  );
};

export default UserFieldSetting;
