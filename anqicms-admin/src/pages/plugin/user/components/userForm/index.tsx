import AttachmentSelect from '@/components/attachment';
import {
  getArchives,
  getCategories,
  pluginGetUserFieldsSetting,
  pluginGetUserGroups,
  pluginGetUserInfo,
  pluginSaveUserInfo,
} from '@/services';
import {
  DeleteOutlined,
  DownOutlined,
  LeftOutlined,
  PlusOutlined,
  RightOutlined,
  UpOutlined,
} from '@ant-design/icons';
import {
  ModalForm,
  ProFormCheckbox,
  ProFormDatePicker,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Divider, Image, Modal, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

export type UserFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  user: any;
};

const UserForm: React.FC<UserFormProps> = (props) => {
  const formRef = useRef<ProFormInstance>();
  const [userFields, setUserFields] = useState<any[]>([]);
  const [user, setUser] = useState<any>({ extra: {} });
  const [categories, setCategories] = useState<any[]>([]);
  const [fetched, setFetched] = useState<boolean>(false);
  const intl = useIntl();
  const [searchArchives, setSearchArchives] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'content.parent_id.empty',
      }),
    },
  ]);
  const [selectedArchives, setSelectedArchives] = useState<any[]>([]);

  const getSelectedArchives = (arcIds: number[]) => {
    if (arcIds.length > 0) {
      // 存在了再处理
      getArchives({
        id: arcIds.join(','),
        limit: 20,
      }).then((res) => {
        if (res.data) {
          setSelectedArchives(res.data);
          setSearchArchives(res.data);
        }
      });
    }
  };

  const initData = async () => {
    let res = await pluginGetUserFieldsSetting();
    let fields = res.data?.fields || [];
    setUserFields(fields);
    pluginGetUserInfo({ id: props.user.id }).then((res) => {
      let data = res.data || { extra: {}, status: 1 };
      if (typeof data.extra === 'undefined' || data.extra === null) {
        data.extra = {};
      }
      if (data.expire_time) {
        data.expire_time = dayjs(data.expire_time * 1000);
      }
      if (data.birthday) {
        data.birthday = dayjs(data.birthday * 1000);
      }
      if (data.avatar_url) {
        data.avatar_url = data.full_avatar_url;
      }
      let arcIds = [];
      // eslint-disable-next-line guard-for-in
      for (let i in fields) {
        let field = fields[i];
        if (
          field.type === 'archive' &&
          data.extra?.[field.field_name]?.value?.length > 0
        ) {
          arcIds.push(...data.extra[field.field_name].value);
        }
      }
      getSelectedArchives(arcIds);
      getCategories({ type: 1 }).then((res) => {
        setCategories(res.data || []);
      });
      setUser(data);
      setFetched(true);
    });
  };

  useEffect(() => {
    initData();
  }, []);

  const onSubmit = async (values: any) => {
    const data = Object.assign(user, values);

    const res = await pluginSaveUserInfo(data);
    message.info(res.msg);

    props.onSubmit();
  };

  const handleCleanExtraField = (field: string) => {
    const extra: any = {};
    extra[field] = { value: '' };

    formRef?.current?.setFieldsValue({ extra });

    delete user.extra[field];
    setUser((prevUser: any) => ({
      ...prevUser,
      extra: {
        ...user.extra,
      },
    }));
  };

  const handleUploadExtraField = (field: string, row: any) => {
    const extra: any = {};
    extra[field] = { value: row.file_path };
    formRef?.current?.setFieldsValue({ extra });
    if (!user.extra) {
      user.extra = {};
    }
    if (!user.extra[field]) {
      user.extra[field] = {};
    }
    user.extra[field].value = row.file_path;

    setUser((prevUser: any) => ({
      ...prevUser,
      extra: {
        ...user.extra,
      },
    }));
  };

  const handleMoveExtraFieldItem = (
    field: string,
    index: number,
    direction: 'up' | 'down',
  ) => {
    if (direction === 'up') {
      if (index <= 0) {
        return;
      }
      const temp = user.extra[field].value[index];
      user.extra[field].value[index] = user.extra[field].value[index - 1];
      user.extra[field].value[index - 1] = temp;
    } else {
      if (index >= user.extra[field].value.length - 1) {
        return;
      }
      const temp = user.extra[field].value[index];
      user.extra[field].value[index] = user.extra[field].value[index + 1];
      user.extra[field].value[index + 1] = temp;
    }

    setUser((prevUser: any) => ({
      ...prevUser,
      extra: {
        ...user.extra,
      },
    }));
  };

  const handleCleanExtraFieldItem = (field: string, index: number) => {
    user.extra[field]?.value?.splice(index, 1);
    const extra: any = {};
    extra[field] = { value: user.extra[field]?.value };
    formRef?.current?.setFieldsValue({ extra });

    setUser((prevUser: any) => ({
      ...prevUser,
      extra: {
        ...prevUser.extra,
        [field]: user.extra[field],
      },
    }));
  };

  const handleUploadExtraFieldItem = (field: string, rows: any) => {
    if (!user.extra[field]) {
      user.extra[field] = { value: [] };
    }
    if (!user.extra[field].value?.length) {
      user.extra[field].value = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in user.extra[field].value) {
        if (user.extra[field].value[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        user.extra[field].value.push(row.file_path);
      }
    }
    const extra: any = {};
    extra[field] = { value: user.extra[field].value };
    formRef?.current?.setFieldsValue({ extra });

    setUser((prevUser: any) => ({
      ...prevUser,
      extra: {
        ...prevUser.extra,
        [field]: user.extra[field],
      },
    }));
  };

  const onAddExtraTextsField = (field: string) => {
    if (!user.extra[field]) {
      user.extra[field] = { value: [] };
    }
    user.extra[field].value.push({
      key: '',
      value: '',
    });
    const extra: any = {};
    extra[field] = { value: user.extra[field].value };
    formRef?.current?.setFieldsValue({ extra });
    setUser((prevUser: any) => ({
      ...prevUser,
      extra: {
        ...user.extra,
      },
    }));
  };

  const onChangeExtraTextsField = (
    field: string,
    idx: number,
    keyName: any,
    value: any,
  ) => {
    if (!user.extra[field].value[idx]) {
      user.extra[field].value[idx] = {};
    }
    user.extra[field].value[idx][keyName] = value;
    const extra: any = {};
    extra[field] = { idx: { keyName: value } };
    formRef?.current?.setFieldsValue({ extra });
    setUser((prevUser: any) => ({
      ...prevUser,
      extra: {
        ...user.extra,
      },
    }));
  };

  const onMoveUpExtraTextsField = (field: string, idx: number) => {
    // 移动
    if (idx > 0) {
      const tmp = user.extra[field].value[idx];
      user.extra[field].value[idx] = user.extra[field].value[idx - 1];
      user.extra[field].value[idx - 1] = tmp;
      const extra: any = {};
      extra[field] = { value: user.extra[field].value };
      formRef?.current?.setFieldsValue({ extra });
      setUser((prevUser: any) => ({
        ...prevUser,
        extra: {
          ...user.extra,
        },
      }));
    }
  };

  const onMoveDownExtraTextsField = (field: string, idx: number) => {
    // 移动
    if (idx < user.extra[field].value.length - 1) {
      const tmp = user.extra[field].value[idx];
      user.extra[field].value[idx] = user.extra[field].value[idx + 1];
      user.extra[field].value[idx + 1] = tmp;
      const extra: any = {};
      extra[field] = { value: user.extra[field].value };
      formRef?.current?.setFieldsValue({ extra });
      setUser((prevUser: any) => ({
        ...prevUser,
        extra: {
          ...user.extra,
        },
      }));
    }
  };

  const onRemoveExtraTextsField = (field: string, idx: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'content.module.field.delete.confirm',
      }),
      content: intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: () => {
        if (user.extra[field].value.length === 1) {
          user.extra[field] = { value: [] };
        } else {
          user.extra[field].value.splice(idx, 1);
        }
        setUser((prevUser: any) => ({
          ...prevUser,
          extra: {
            ...user.extra,
          },
        }));
        const extra: any = {};
        extra[field] = { value: user.extra[field].value };
        formRef?.current?.setFieldsValue({ extra });
      },
    });
  };

  const onSearchArchives = (e: any) => {
    getArchives({ title: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [];
      if (selectedArchives) {
        searchItems.push(...selectedArchives);
      } else {
        searchItems.push({
          id: 0,
          title: intl.formatMessage({
            id: 'content.parent_id.empty',
          }),
        });
      }
      setSearchArchives(searchItems.concat(res.data || []));
    });
  };

  const handleDeleteAvatarUrl = () => {
    user.avatar_url = '';
    setUser((prevUser: any) => ({
      ...prevUser,
      avatar_url: '',
    }));
  };

  const handleUploadAvatarUrl = (row: any) => {
    user.avatar_url = row.file_path;
    setUser((prevUser: any) => ({
      ...prevUser,
      avatar_url: row.file_path,
    }));
  };

  return fetched ? (
    <ModalForm
      width={600}
      title={
        props.user?.id
          ? intl.formatMessage({ id: 'plugin.user.edit' })
          : intl.formatMessage({ id: 'plugin.user.add' })
      }
      open={props.open}
      layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      formRef={formRef}
      initialValues={user}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <ProFormText label={intl.formatMessage({ id: 'plugin.user.avatar_url' })}>
        {user.avatar_url ? (
          <div className="ant-upload-item">
            <Image
              preview={{
                src: user.avatar_url,
              }}
              src={user.avatar_url}
            />
            <span className="delete" onClick={() => handleDeleteAvatarUrl()}>
              <DeleteOutlined />
            </span>
          </div>
        ) : (
          <AttachmentSelect
            onSelect={(row) => handleUploadAvatarUrl(row)}
            open={false}
          >
            <div className="ant-upload-item">
              <div className="add">
                <PlusOutlined />
                <div style={{ marginTop: 8 }}>
                  <FormattedMessage id="plugin.pay.upload" />
                </div>
              </div>
            </div>
          </AttachmentSelect>
        )}
      </ProFormText>
      <ProFormText
        name="user_name"
        label={intl.formatMessage({ id: 'plugin.user.user-name' })}
      />
      <ProFormText
        name="first_name"
        label={intl.formatMessage({ id: 'plugin.user.first-name' })}
      />
      <ProFormText
        name="last_name"
        label={intl.formatMessage({ id: 'plugin.user.last-name' })}
      />
      <ProFormDatePicker
        name="birthday"
        label={intl.formatMessage({ id: 'plugin.user.birthday' })}
        width="lg"
        transform={(value, namePath) => {
          return { [namePath]: dayjs(value).unix() };
        }}
      />
      <ProFormText
        name="real_name"
        label={intl.formatMessage({ id: 'plugin.user.real-name' })}
      />
      <ProFormText
        name="phone"
        label={intl.formatMessage({ id: 'plugin.user.phone' })}
      />
      <ProFormText
        name="email"
        label={intl.formatMessage({ id: 'plugin.user.email' })}
      />
      <ProFormText
        name="password"
        label={intl.formatMessage({ id: 'plugin.user.password' })}
        extra={intl.formatMessage({ id: 'plugin.user.password.description' })}
      />
      <ProFormTextArea
        name="introduce"
        label={intl.formatMessage({ id: 'plugin.user.introduce' })}
      />
      <ProFormRadio.Group
        name="is_retailer"
        label={intl.formatMessage({ id: 'plugin.user.is-retailer' })}
        options={[
          {
            label: intl.formatMessage({ id: 'plugin.user.is-retailer.no' }),
            value: 0,
          },
          {
            label: intl.formatMessage({ id: 'plugin.user.is-retailer.yes' }),
            value: 1,
          },
        ]}
      />
      <ProFormText
        name="invite_code"
        label={intl.formatMessage({ id: 'plugin.user.invite-code' })}
        extra={intl.formatMessage({
          id: 'plugin.user.invite-code.description',
        })}
      />
      <ProFormDigit
        name="parent_id"
        label={intl.formatMessage({ id: 'plugin.user.parent.user-id' })}
      />
      <ProFormSelect
        label={intl.formatMessage({ id: 'plugin.user.group' })}
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
        }}
      />
      <ProFormDatePicker
        name="expire_time"
        label={intl.formatMessage({ id: 'plugin.user.expire' })}
        extra={intl.formatMessage({ id: 'plugin.user.expire.description' })}
        width="lg"
        transform={(value, namePath) => {
          return { [namePath]: dayjs(value).unix() };
        }}
      />
      <ProFormRadio.Group
        label={intl.formatMessage({ id: 'website.status' })}
        name="status"
        options={[
          {
            label: intl.formatMessage({ id: 'plugin.user.normal' }),
            value: 1,
          },
          {
            label: intl.formatMessage({ id: 'plugin.user.pending' }),
            value: 0,
          },
          {
            label: intl.formatMessage({ id: 'plugin.user.blocked' }),
            value: -1,
          },
        ]}
      />
      <Divider>
        <FormattedMessage id="plugin.user.extra-fields" />
      </Divider>
      {userFields.map((item: any) =>
        item.type === 'text' ? (
          <ProFormText
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
            required={item.required ? true : false}
            placeholder={
              item.content &&
              intl.formatMessage({ id: 'plugin.user.extra-fields.default' }) +
                item.content
            }
          />
        ) : item.type === 'number' ? (
          <ProFormDigit
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
            required={item.required ? true : false}
            placeholder={
              item.content &&
              intl.formatMessage({ id: 'plugin.user.extra-fields.default' }) +
                item.content
            }
          />
        ) : item.type === 'textarea' ? (
          <ProFormTextArea
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
            required={item.required ? true : false}
            placeholder={
              item.content &&
              intl.formatMessage({ id: 'plugin.user.extra-fields.default' }) +
                item.content
            }
          />
        ) : item.type === 'editor' ? (
          <ProFormTextArea
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
            required={item.required ? true : false}
            placeholder={
              item.content &&
              intl.formatMessage({ id: 'plugin.user.extra-fields.default' }) +
                item.content
            }
          />
        ) : item.type === 'radio' ? (
          <ProFormRadio.Group
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
            request={async () => {
              const tmpData = item.content.split('\n');
              const data = [];
              for (const item1 of tmpData) {
                data.push({ label: item1, value: item1 });
              }
              return data;
            }}
          />
        ) : item.type === 'checkbox' ? (
          <ProFormCheckbox.Group
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
            request={async () => {
              const tmpData = item.content.split('\n');
              const data = [];
              for (const item1 of tmpData) {
                data.push({ label: item1, value: item1 });
              }
              return data;
            }}
          />
        ) : item.type === 'select' ? (
          <ProFormSelect
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
            request={async () => {
              const tmpData = item.content.split('\n');
              const data = [];
              for (const item1 of tmpData) {
                data.push({ label: item1, value: item1 });
              }
              return data;
            }}
          />
        ) : item.type === 'image' ? (
          <ProFormText
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
          >
            {user.extra?.[item.field_name]?.value ? (
              <div className="ant-upload-item">
                <Image
                  preview={{
                    src: user.extra[item.field_name]?.value,
                  }}
                  src={user.extra[item.field_name]?.value}
                />
                <span
                  className="delete"
                  onClick={() => handleCleanExtraField(item.field_name)}
                >
                  <DeleteOutlined />
                </span>
              </div>
            ) : (
              <AttachmentSelect
                onSelect={(row) => handleUploadExtraField(item.field_name, row)}
                open={false}
              >
                <div className="ant-upload-item">
                  <div className="add">
                    <PlusOutlined />
                    <div style={{ marginTop: 8 }}>
                      <FormattedMessage id="plugin.pay.upload" />
                    </div>
                  </div>
                </div>
              </AttachmentSelect>
            )}
          </ProFormText>
        ) : item.type === 'images' ? (
          <ProFormText
            name={['extra', item.field_name, 'value']}
            label={item.name}
          >
            {user.extra?.[item.field_name]?.value?.length
              ? user.extra[item.field_name].value.map(
                  (inner: string, idx: number) => (
                    <div className="ant-upload-item" key={idx}>
                      <Image
                        preview={{
                          src: inner,
                        }}
                        src={inner}
                      />
                      <div className="ant-upload-item-action">
                        <Tag
                          onClick={() =>
                            handleMoveExtraFieldItem(item.field_name, idx, 'up')
                          }
                        >
                          <LeftOutlined />
                        </Tag>
                        <Tag
                          color="red"
                          onClick={() =>
                            handleCleanExtraFieldItem(item.field_name, idx)
                          }
                        >
                          <DeleteOutlined />
                        </Tag>
                        <Tag
                          onClick={() =>
                            handleMoveExtraFieldItem(
                              item.field_name,
                              idx,
                              'down',
                            )
                          }
                        >
                          <RightOutlined />
                        </Tag>
                      </div>
                    </div>
                  ),
                )
              : null}
            <AttachmentSelect
              onSelect={(rows) =>
                handleUploadExtraFieldItem(item.field_name, rows)
              }
              open={false}
              multiple={true}
            >
              <div className="ant-upload-item">
                <div className="add">
                  <PlusOutlined />
                  <div style={{ marginTop: 8 }}>
                    <FormattedMessage id="setting.system.upload" />
                  </div>
                </div>
              </div>
            </AttachmentSelect>
          </ProFormText>
        ) : item.type === 'file' ? (
          <ProFormText
            key={item.field_name}
            name={['extra', item.field_name, 'value']}
            label={item.name}
          >
            {user.extra?.[item.field_name]?.value ? (
              <div className="ant-upload-item ant-upload-file">
                <div style={{ height: 60 }}>
                  <a
                    href={user.extra[item.field_name]?.value}
                    target="_blank"
                    rel="noreferrer"
                  >
                    {user.extra[item.field_name]?.value}
                  </a>
                </div>
                <span
                  className="delete"
                  onClick={() => handleCleanExtraField(item.field_name)}
                >
                  <DeleteOutlined />
                </span>
              </div>
            ) : (
              <AttachmentSelect
                onSelect={(row) => handleUploadExtraField(item.field_name, row)}
                open={false}
              >
                <Button>
                  <FormattedMessage id="plugin.pay.upload" />
                </Button>
              </AttachmentSelect>
            )}
          </ProFormText>
        ) : item.type === 'texts' ? (
          <ProFormText label={item.name}>
            <div className="text-groups">
              <div className="text-group">
                <div className="text-key">Key</div>
                <div className="text-value">Value</div>
                <div className="text-action"></div>
              </div>
              {user.extra?.[item.field_name]?.value?.length
                ? user.extra[item.field_name].value.map(
                    (inner: any, idx: number) => (
                      <div className="text-group" key={idx}>
                        <div className="text-key">
                          <ProFormText
                            name={[
                              'extra',
                              item.field_name,
                              'value',
                              idx,
                              'key',
                            ]}
                            fieldProps={{
                              onChange: (e: any) => {
                                onChangeExtraTextsField(
                                  item.field_name,
                                  idx,
                                  'key',
                                  e.target.value,
                                );
                              },
                            }}
                          />
                        </div>
                        <div className="text-value">
                          <ProFormText
                            name={[
                              'extra',
                              item.field_name,
                              'value',
                              idx,
                              'value',
                            ]}
                            fieldProps={{
                              onChange: (e: any) => {
                                onChangeExtraTextsField(
                                  item.field_name,
                                  idx,
                                  'value',
                                  e.target.value,
                                );
                              },
                            }}
                          />
                        </div>
                        <div className="text-action">
                          <Tag
                            onClick={() =>
                              onMoveUpExtraTextsField(item.field_name, idx)
                            }
                          >
                            <UpOutlined />
                          </Tag>
                          <Tag
                            onClick={() =>
                              onMoveDownExtraTextsField(item.field_name, idx)
                            }
                          >
                            <DownOutlined />
                          </Tag>
                          <Tag
                            color="red"
                            onClick={() =>
                              onRemoveExtraTextsField(item.field_name, idx)
                            }
                          >
                            <DeleteOutlined />
                          </Tag>
                        </div>
                      </div>
                    ),
                  )
                : null}
              <div className="text-group">
                <div className="text-key">
                  <Tag
                    color="blue"
                    className="add-line"
                    onClick={() => onAddExtraTextsField(item.field_name)}
                  >
                    {intl.formatMessage({
                      id: 'content.param.add-line',
                    })}
                  </Tag>
                </div>
              </div>
            </div>
          </ProFormText>
        ) : item.type === 'archive' ? (
          <ProFormText label={item.name}>
            <ProFormSelect
              name={['extra', item.field_name, 'value']}
              showSearch
              mode="multiple"
              options={searchArchives.map((a: any) => ({
                title: a.title,
                label: a.title,
                value: a.id,
              }))}
              fieldProps={{
                onSearch: (e) => {
                  onSearchArchives(e);
                },
              }}
            />
          </ProFormText>
        ) : item.type === 'category' ? (
          <ProFormText label={item.name}>
            <ProFormSelect
              showSearch
              name={['extra', item.field_name, 'value']}
              mode={'single'}
              options={[
                {
                  title: intl.formatMessage({
                    id: 'content.please-select',
                  }),
                  value: 0,
                  status: 1,
                },
              ]
                .concat(categories)
                .map((cat: any) => ({
                  title: cat.title,
                  label: (
                    <div title={cat.title}>
                      {cat.parents?.length > 0 ? (
                        <span className="text-muted">
                          {cat.parents
                            ?.map((parent: any) => parent.title)
                            .join(' > ')}
                          {' > '}
                        </span>
                      ) : (
                        ''
                      )}
                      {cat.title}
                    </div>
                  ),
                  value: cat.id,
                  disabled: cat.status !== 1,
                }))}
              fieldProps={{
                showSearch: true,
                filterOption: (input: string, option: any) =>
                  (option?.title ?? option?.label)
                    .toLowerCase()
                    .includes(input.toLowerCase()),
              }}
            />
          </ProFormText>
        ) : (
          ''
        ),
      )}
    </ModalForm>
  ) : null;
};

export default UserForm;
