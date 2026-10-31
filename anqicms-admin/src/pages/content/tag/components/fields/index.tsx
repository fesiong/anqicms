import { getTagFields, saveTagFields } from '@/services';
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
import React, { useEffect, useRef, useState } from 'react';

export type TagFieldsFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
};

let submitting = false;

const TagFieldsForm: React.FC<TagFieldsFormProps> = (props) => {
  const actionArchiveRef = useRef<ActionType>();
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentField, setCurrentField] = useState<any>({});
  const [tagFields, setTagFields] = useState<any>([]);
  const [fetched, setFetched] = useState<boolean>(false);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getTagFields({});
    let setting = res.data || [];
    setTagFields(setting);
    setFetched(true);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleRemoveItem = (record: any, index: number) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.module.field.delete.confirm' }),
      content: intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: async () => {
        tagFields.splice(index, 1);
        setTagFields([].concat(tagFields));
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
    for (let i in tagFields) {
      if (tagFields[i].field_name === values.field_name) {
        exists = true;
        tagFields[i] = values;
      }
    }
    if (!exists) {
      tagFields.push(values);
    }
    setTagFields([].concat(tagFields));
    if (actionArchiveRef.current) {
      actionArchiveRef.current.reload();
    }
    setEditVisible(false);
  };

  const handleSaveSetting = async () => {
    if (submitting) {
      return;
    }
    submitting = true;
    let hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveTagFields(tagFields)
      .then((res) => {
        if (res.code === 0) {
          message.success(res.msg);
          setEditVisible(false);
          if (actionArchiveRef.current) {
            actionArchiveRef.current.reload();
          }
          props.onSubmit();
        } else {
          message.error(res.msg);
        }
      })
      .finally(() => {
        submitting = false;
        hide();
      });
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
      <Modal
        width={1000}
        title={intl.formatMessage({ id: 'content.tag.fields' })}
        open={props.open}
        onCancel={() => {
          props.onCancel();
        }}
        onOk={() => {
          handleSaveSetting();
        }}
      >
        {fetched && (
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
                  data: tagFields,
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
        )}
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
          <ProFormRadio.Group
            name="follow_level"
            label={intl.formatMessage({ id: 'content.read-level.name' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.module.field.level.none',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'content.module.field.level.follow',
                }),
                value: true,
              },
            ]}
          />
          <ProFormRadio.Group
            name="is_filter"
            label={intl.formatMessage({ id: 'content.module.field.isfilter' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isfilter.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isfilter.yes',
                }),
                value: true,
              },
            ]}
            extra={intl.formatMessage({
              id: 'content.module.field.isfilter.description',
            })}
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

export default TagFieldsForm;
