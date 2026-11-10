import { getDesignList, saveWebsiteInfo } from '@/services';
import {
  ModalForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Collapse, RadioChangeEvent, message } from 'antd';
import React, { useEffect, useState } from 'react';

const { Panel } = Collapse;

let submiting = false;
export type WebsiteFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  website: any;
  rootPath: string;
};

const WebsiteForm: React.FC<WebsiteFormProps> = (props) => {
  const intl = useIntl();
  const [userDefault, setUseDefault] = useState<boolean>(false);

  useEffect(() => {
    if (props.website.mysql && props.website.mysql.use_default) {
      setUseDefault(true);
    }
  }, []);

  const handleChangeUse = (e: RadioChangeEvent) => {
    setUseDefault(e.target.value);
  };

  const onSubmitEdit = async (values: any) => {
    if (props.website.id === 1) {
      // 自己无法禁用自己
      values.status = 1;
    }
    if (submiting) {
      return;
    }
    submiting = true;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    values.status = Number(values.status);
    const postData = Object.assign(props.website, values);
    saveWebsiteInfo(postData)
      .then((res) => {
        if (res.code !== 0) {
          message.error(res.msg);
        } else {
          message.info(res.msg);

          props.onSubmit();
        }
      })
      .finally(() => {
        submiting = false;
        hide();
      });
  };

  return (
    <ModalForm
      width={600}
      title={
        props.website.id > 0
          ? intl.formatMessage({ id: 'website.edit' })
          : intl.formatMessage({ id: 'website.add' })
      }
      open={props.open}
      layout="horizontal"
      modalProps={{
        maskClosable: false,
      }}
      initialValues={props.website}
      onFinish={onSubmitEdit}
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
    >
      {props.website.id > 0 && (
        <ProFormDigit
          name="id"
          label={intl.formatMessage({ id: 'website.id' })}
          readonly
        />
      )}
      <ProFormText
        name="name"
        label={intl.formatMessage({ id: 'website.name' })}
      />
      <ProFormText
        name="root_path"
        label={intl.formatMessage({ id: 'website.root-path' })}
        disabled={props.website.id === 1}
        placeholder={intl.formatMessage({
          id: 'website.root-path.placeholder',
        })}
        extra={
          <div>
            {intl.formatMessage({ id: 'website.root-path.prefix' })}
            {props.rootPath}
            <br />
            {intl.formatMessage({ id: 'website.root-path.description' })}
          </div>
        }
      />
      <ProFormText
        name="base_url"
        label={intl.formatMessage({ id: 'setting.system.base-url' })}
        placeholder={intl.formatMessage({
          id: 'website.base-url.placeholder',
        })}
        extra={intl.formatMessage({ id: 'website.base-url.description' })}
      />
      <ProFormText
        name="admin_user"
        label={intl.formatMessage({ id: 'website.admin-user' })}
      />
      <ProFormText.Password
        name="admin_password"
        label={intl.formatMessage({ id: 'website.admin-password' })}
        placeholder={intl.formatMessage({
          id: 'website.admin-password.description',
        })}
      />
      {props.website.id !== 1 && (
        <Collapse defaultActiveKey={props.website.id > 0 ? [] : [1]} ghost>
          <Panel
            header={intl.formatMessage({ id: 'website.db.header' })}
            key="1"
          >
            <ProFormText
              name={['mysql', 'database']}
              label={intl.formatMessage({ id: 'website.db.database' })}
              placeholder={intl.formatMessage({
                id: 'website.db.database.description',
              })}
            />
            <ProFormRadio.Group
              label={intl.formatMessage({ id: 'website.db.use-default' })}
              name={['mysql', 'use_default']}
              options={[
                {
                  label: intl.formatMessage({
                    id: 'website.db.use-default.new',
                  }),
                  value: false,
                },
                {
                  label: intl.formatMessage({
                    id: 'website.db.use-default.default',
                  }),
                  value: true,
                },
              ]}
              fieldProps={{
                onChange: handleChangeUse,
              }}
            />
            {!userDefault && (
              <>
                <ProFormText
                  name={['mysql', 'host']}
                  label={intl.formatMessage({ id: 'website.db.host' })}
                  placeholder={intl.formatMessage({
                    id: 'website.db.host.description',
                  })}
                />
                <ProFormDigit
                  name={['mysql', 'port']}
                  label={intl.formatMessage({ id: 'website.db.port' })}
                  placeholder={intl.formatMessage({
                    id: 'website.db.port.description',
                  })}
                />
                <ProFormText
                  name={['mysql', 'user']}
                  label={intl.formatMessage({ id: 'website.db.user' })}
                />
                <ProFormText
                  name={['mysql', 'password']}
                  label={intl.formatMessage({ id: 'website.db.password' })}
                />
              </>
            )}
            {!props.website.id && (
              <>
                <ProFormSelect
                  label={intl.formatMessage({ id: 'website.db.template' })}
                  showSearch
                  name="template"
                  request={async () => {
                    const res = await getDesignList({});
                    const data = res.data || [];
                    for (const i in data) {
                      if (data.hasOwnProperty(i)) {
                        data[i].label =
                          data[i].name + '(' + data[i].package + ')';
                      }
                    }
                    return data;
                  }}
                  fieldProps={{
                    fieldNames: {
                      label: 'label',
                      value: 'package',
                    },
                  }}
                  extra={intl.formatMessage({
                    id: 'website.db.template.description',
                  })}
                />
                <ProFormCheckbox
                  name="preview_data"
                  label={intl.formatMessage({
                    id: 'website.db.preview-data',
                  })}
                  extra={intl.formatMessage({
                    id: 'website.db.preview-data.description',
                  })}
                />
              </>
            )}
          </Panel>
        </Collapse>
      )}
      <ProFormRadio.Group
        label={intl.formatMessage({ id: 'website.status' })}
        name="status"
        readonly={props.website.id === 1}
        options={[
          {
            label: intl.formatMessage({ id: 'setting.content.notenable' }),
            value: 0,
          },
          {
            label: intl.formatMessage({ id: 'setting.content.enable' }),
            value: 1,
          },
        ]}
      />
    </ModalForm>
  );
};

export default WebsiteForm;
