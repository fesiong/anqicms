import { useVipModal } from '@/components/vipModal';
import { getSiteInfo, getWebsiteList } from '@/services';
import {
  pluginGetGuestbookSetting,
  pluginSaveGuestbookSetting,
} from '@/services/plugin/guestbook';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, Link, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import './index.less';

export type GuestbookSettingProps = {
  children?: React.ReactNode;
};

const GuestbookSetting: React.FC<GuestbookSettingProps> = (props) => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const formRef = useRef<ProFormInstance>();
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [setting, setSetting] = useState<any>({ fields: [] });
  const [fetched, setFetched] = useState<boolean>(false);
  const [siteInfo, setSiteInfo] = useState<any>({});
  const intl = useIntl();

  const getSetting = async () => {
    getSiteInfo({}).then((res) => {
      setSiteInfo(res?.data || {});
    });
    const res = await pluginGetGuestbookSetting();
    let setting = res.data || { fields: [] };
    setSetting(setting);
    setFetched(true);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleRemoveItem = (index: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.guestbook.field.delete.confirm',
      }),
      content: intl.formatMessage({
        id: 'plugin.guestbook.field.delete.confirm.content',
      }),
      onOk: async () => {
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
    if (!setting.fields) {
      setting.fields = [];
    }
    let exists = false;
    for (let i in setting.fields) {
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
    if (actionRef.current) {
      actionRef.current.reload();
    }
    setEditVisible(false);
  };

  const handleChangeReturnMessage = (e: any) => {
    setting.return_message = e.target.value;
    setSetting(setting);
  };

  const handleChangePushWay = (e: any) => {
    setSetting({
      ...setting,
      push_way: e.target.value,
    });
  };

  const handleSaveSetting = async (values: any) => {
    let postData = {
      ...setting,
      ...values,
    };
    let res = await pluginSaveGuestbookSetting(postData);

    if (res.code === 0) {
      message.success(res.msg);
      setEditVisible(false);
      if (actionRef.current) {
        actionRef.current.reload();
      }
    } else {
      message.error(res.msg);
    }
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
      title: intl.formatMessage({ id: 'content.module.field.isrequired' }),
      dataIndex: 'required',

      valueEnum: {
        false: {
          text: intl.formatMessage({
            id: 'content.module.field.isrequired.no',
          }),
          status: 'Default',
        },
        true: {
          text: intl.formatMessage({
            id: 'content.module.field.isrequired.yes',
          }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      render: (text: any, record, index) => (
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
                  handleRemoveItem(index);
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
      <VipModal />
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <ModalForm
        width={800}
        title={intl.formatMessage({ id: 'plugin.guestbook.setting' })}
        open={visible}
        onOpenChange={(flag) => {
          setVisible(flag);
          if (flag) {
            formRef.current?.setFieldsValue(setting);
          }
        }}
        formRef={formRef}
        layout="horizontal"
        labelCol={{ span: 4 }}
        wrapperCol={{ span: 14 }}
        onFinish={handleSaveSetting}
      >
        <div>
          <ProFormText
            name="return_message"
            label={intl.formatMessage({
              id: 'plugin.guestbook.return-message',
            })}
            placeholder={intl.formatMessage({
              id: 'plugin.guestbook.return-message.placeholder',
            })}
            extra={intl.formatMessage({
              id: 'plugin.guestbook.return-message.description',
            })}
            fieldProps={{
              defaultValue: setting.return_message,
              onChange: handleChangeReturnMessage,
            }}
          />
          <ProFormRadio.Group
            name="push_way"
            label={intl.formatMessage({ id: 'plugin.guestbook.push-way' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.guestbook.push-way.email',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.guestbook.push-way.site',
                }),
                value: 1,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.guestbook.push-way.api',
                }),
                value: 2,
              },
            ]}
            disabled={isVip === false}
            fieldProps={{
              onChange: handleChangePushWay,
            }}
            extra={
              !isVip ? (
                <div
                  className="link"
                  onClick={() => {
                    checkVip(() => {});
                  }}
                >
                  <FormattedMessage id="plugin.guestbook.vip-tip" />
                </div>
              ) : null
            }
          />
          {setting.push_way === 0 && (
            <ProFormText
              label={intl.formatMessage({
                id: 'plugin.guestbook.email-setting',
              })}
              readonly
            >
              <div>
                <FormattedMessage id="plugin.guestbook.email-tips.before" />
                <Link to={'/plugin/channel/sendmail'}>
                  <FormattedMessage id="plugin.guestbook.email-tips.link" />
                </Link>
                <FormattedMessage id="plugin.guestbook.email-tips.after" />
              </div>
            </ProFormText>
          )}
          {setting.push_way === 1 && (
            <ProFormSelect
              name="site_id"
              label={intl.formatMessage({ id: 'plugin.guestbook.select-site' })}
              request={async () => {
                const res = await getWebsiteList();
                return (
                  res.data
                    ?.filter((item: any) => item.status === 1)
                    .map((item: any) => ({
                      label:
                        item.name +
                        '(ID: ' +
                        item.id +
                        ',URL: ' +
                        item.base_url +
                        ')',
                      value: item.id,
                      disabled: item.id === siteInfo.id,
                    })) || []
                );
              }}
            />
          )}
          {setting.push_way === 2 && (
            <div>
              <ProFormText
                name="api_url"
                label={intl.formatMessage({ id: 'plugin.guestbook.api-url' })}
              />
              <ProFormText label="Header">
                <Space className="no-margin">
                  <ProFormText
                    name="header_key"
                    addonBefore="Key"
                    width={150}
                  />
                  <ProFormText
                    width={200}
                    name="header_value"
                    addonBefore="Value"
                  />
                </Space>
              </ProFormText>
              <ProFormRadio.Group
                name="api_method"
                label={intl.formatMessage({ id: 'plugin.guestbook.submit-way' })}
                options={[
                  { label: 'JSON', value: 'json' },
                  { label: 'Form-Data', value: 'formdata' },
                  { label: 'Query(GET)', value: 'query' },
                ]}
              />
            </div>
          )}
        </div>
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
          columns={columns}
          pagination={false}
        />
      </ModalForm>
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
              id: 'content.module.field.name.description',
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
              file: intl.formatMessage({
                id: 'content.module.field.type.file',
              }),
            }}
          />
          <ProFormRadio.Group
            name="required"
            label={intl.formatMessage({
              id: 'content.module.field.isrequired',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isrequired.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isrequired.yes',
                }),
                value: true,
              },
            ]}
          />
          <ProFormTextArea
            label={intl.formatMessage({ id: 'content.category.default' })}
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

export default GuestbookSetting;
