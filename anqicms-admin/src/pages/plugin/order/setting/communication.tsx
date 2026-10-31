import {
  pluginGetCommunicationSetting,
  pluginGetOrderSetting,
  pluginSaveCommunicationSetting,
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
import { Alert, Button, Divider, Modal, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

export type OrderSettingProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const CommunicationOrderSetting: React.FC<OrderSettingProps> = (props) => {
  const actionArchiveRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [setting, setSetting] = useState<any>({ order_fields: [] });
  const [orderFields, setOrderFields] = useState<any>([]);

  const intl = useIntl();

  useEffect(() => {
    pluginGetCommunicationSetting().then((res) => {
      setSetting(res.data);
      setOrderFields(res.data.order_fields || []);
      setFetched(true);
    });
  }, []);

  const handleSaveSetting = async (values: any) => {
    const postData = {
      ...values,
      order_fields: orderFields,
    };
    const res = await pluginSaveCommunicationSetting(postData);

    if (res.code === 0) {
      message.success(res.msg || 'success');
      setVisible(false);
    } else {
      message.error(res.msg);
    }
  };

  const handleRemoveItem = (record: any, index: number) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.module.field.delete.confirm' }),
      content: intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: async () => {
        orderFields.splice(index, 1);
        setOrderFields([].concat(orderFields));
        if (actionArchiveRef.current) {
          actionArchiveRef.current.reload();
        }
      },
    });
  };

  const handleSaveField = async (values: any) => {
    let reg = /^[a-z][0-9a-z_]+$/;
    if (!values.field_name || !reg.test(values.field_name)) {
      message.error(intl.formatMessage({ id: 'content.module.field.error' }));
      return;
    }
    let exists = false;
    for (let i in orderFields) {
      if (orderFields[i].field_name === values.field_name) {
        exists = true;
        orderFields[i] = values;
      }
    }
    if (!exists) {
      orderFields.push(values);
    }
    setOrderFields([].concat(orderFields));
    if (actionArchiveRef.current) {
      actionArchiveRef.current.reload();
    }
    setEditVisible(false);
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
          {!record.is_system && (
            <>
              <a
                onClick={() => {
                  setCurrentField(record);
                  setEditVisible(true);
                }}
              >
                <FormattedMessage id="setting.action.edit" />
              </a>
              <a
                className="text-red"
                onClick={() => {
                  handleRemoveItem(record, index);
                }}
              >
                <FormattedMessage id="setting.system.delete" />
              </a>
            </>
          )}
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
      {fetched && (
        <ModalForm
          width={800}
          title={intl.formatMessage({ id: 'plugin.order.setting' })}
          open={visible}
          onOpenChange={(flag) => {
            setVisible(flag);
          }}
          request={async () => {
            const res = await pluginGetOrderSetting();
            return res.data;
          }}
          initialValues={setting}
          layout="horizontal"
          onFinish={async (values) => {
            handleSaveSetting(values);
          }}
        >
          <Alert
            message={intl.formatMessage({
              id: 'plugin.order.communication.setting.tips',
            })}
            type="info"
          />
          <Divider>
            <FormattedMessage
              id={'plugin.order.communication.setting.summited-template'}
            />
          </Divider>
          <ProFormText
            name={['summited_template', 'subject']}
            label={intl.formatMessage({
              id: 'plugin.order.communication.setting.template.subject',
            })}
          />
          <ProFormTextArea
            name={['summited_template', 'content']}
            label={intl.formatMessage({
              id: 'plugin.order.communication.setting.template.content',
            })}
            fieldProps={{
              rows: 5,
            }}
          />
          <Divider>
            <FormattedMessage
              id={'plugin.order.communication.setting.quote-template'}
            />
          </Divider>
          <ProFormText
            name={['quote_template', 'subject']}
            label={intl.formatMessage({
              id: 'plugin.order.communication.setting.template.subject',
            })}
          />
          <ProFormTextArea
            name={['quote_template', 'content']}
            label={intl.formatMessage({
              id: 'plugin.order.communication.setting.template.content',
            })}
            fieldProps={{
              rows: 5,
            }}
          />
          <Divider>
            <FormattedMessage
              id={'plugin.order.communication.setting.reply-template'}
            />
          </Divider>
          <ProFormText
            name={['quote_template', 'subject']}
            label={intl.formatMessage({
              id: 'plugin.order.communication.setting.template.subject',
            })}
          />
          <ProFormTextArea
            name={['quote_template', 'content']}
            label={intl.formatMessage({
              id: 'plugin.order.communication.setting.template.content',
            })}
            fieldProps={{
              rows: 5,
            }}
          />
          <Divider>
            <FormattedMessage
              id={'plugin.order.communication.setting.order-fields'}
            />
          </Divider>
          <div>
            <ProTable<any>
              key="fields-table"
              rowKey="name"
              search={false}
              actionRef={actionArchiveRef}
              toolBarRender={() => [
                <Button
                  key="add"
                  type="primary"
                  onClick={() => {
                    setCurrentField({
                      type: 'text',
                      required: false,
                      follow_level: false,
                      is_filter: false,
                    });
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
                  data: orderFields,
                  success: true,
                };
              }}
              columnsState={{
                persistenceKey: 'module-fields-table',
                persistenceType: 'localStorage',
              }}
              columns={columns}
              pagination={false}
            />
          </div>
        </ModalForm>
      )}
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
            extra={intl.formatMessage({
              id: 'content.module.field.field-name.description',
            })}
          />
          <ProFormRadio.Group
            name="type"
            label={intl.formatMessage({ id: 'content.module.field.type' })}
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
              date: intl.formatMessage({
                id: 'content.module.field.type.date',
              }),
              time: intl.formatMessage({
                id: 'content.module.field.type.time',
              }),
              datetime: intl.formatMessage({
                id: 'content.module.field.type.datetime',
              }),
              color: intl.formatMessage({
                id: 'content.module.field.type.color',
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

export default CommunicationOrderSetting;
