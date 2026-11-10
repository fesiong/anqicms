import {
  pluginGetPayAccountInfo,
  pluginPayUploadFile,
  pluginSavePayAccount,
} from '@/services';
import {
  ModalForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormFieldSet,
  ProFormInstance,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Divider, Upload, message } from 'antd';

import React, { useEffect, useRef, useState } from 'react';

export type PayFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  account: any;
};

const PaymentAccountForm: React.FC<PayFormProps> = (props) => {
  const formRef = useRef<ProFormInstance>();
  const [account, setAccount] = useState<any>({ pay_config: {} });
  const [payWay, setPayWay] = useState<any>('');
  const [fetched, setFetched] = useState<boolean>(false);
  const [showMore, setShowMore] = useState<boolean>(false);
  const intl = useIntl();

  // plugin pay index 也有一份
  const PayWayOptions = [
    {
      value: 'wechat',
      label: intl.formatMessage({ id: 'plugin.pay.wechat' }),
    },
    {
      value: 'weapp',
      label: intl.formatMessage({ id: 'plugin.pay.weapp' }),
    },
    {
      value: 'alipay',
      label: intl.formatMessage({ id: 'plugin.pay.alipay' }),
    },
    {
      value: 'paypal',
      label: intl.formatMessage({ id: 'plugin.pay.paypal' }),
    },
    {
      value: 'stripe',
      label: intl.formatMessage({ id: 'plugin.pay.stripe' }),
    },
    {
      value: 'balance',
      label: intl.formatMessage({ id: 'plugin.pay.balance' }),
    },
    {
      value: 'offline',
      label: intl.formatMessage({ id: 'plugin.pay.offline' }),
    },
  ];

  const initData = async () => {
    if (props.account.id > 0) {
      pluginGetPayAccountInfo({ id: props.account.id }).then((res) => {
        let data = res.data || { pay_config: {} };
        if (!data.pay_config) {
          data.pay_config = {};
        }
        data.min_amount =
          data.min_amount > 0 ? data.min_amount / 100 : data.min_amount;
        data.max_amount =
          data.max_amount > 0 ? data.max_amount / 100 : data.max_amount;
        data.daily_amount_limit =
          data.daily_amount_limit > 0
            ? data.daily_amount_limit / 100
            : data.daily_amount_limit;
        data.monthly_amount_limit =
          data.monthly_amount_limit > 0
            ? data.monthly_amount_limit
            : data.monthly_amount_limit;

        setAccount(data);
        setFetched(true);
      });
    } else {
      setAccount({ pay_config: {} });
      setFetched(true);
    }
  };

  useEffect(() => {
    initData();
  }, []);

  const onSubmit = async (values: any) => {
    const data = {
      ...account,
      ...values,
    };
    data.min_amount = Number((values.min_amount * 100).toFixed(0));
    data.max_amount = Number((values.max_amount * 100).toFixed(0));
    data.daily_amount_limit = Number(
      (values.daily_amount_limit * 100).toFixed(0),
    );
    data.monthly_amount_limit = Number(
      (values.monthly_amount_limit * 100).toFixed(0),
    );

    const res = await pluginSavePayAccount(data);
    message.info(res.msg);

    props.onSubmit();
  };

  const handleUploadFile = (field: string, e: any) => {
    const formData = new FormData();
    formData.append('file', e.file);
    formData.append('name', field);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginPayUploadFile(formData)
      .then((res) => {
        message.success(res.msg);
        account[field] = res.data;
        setAccount({ ...account });
      })
      .finally(() => {
        hide();
      });
  };

  return fetched ? (
    <ModalForm
      width={800}
      title={
        props.account?.id
          ? intl.formatMessage({ id: 'plugin.pay.edit' })
          : intl.formatMessage({ id: 'plugin.pay.add' })
      }
      open={props.open}
      layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      formRef={formRef}
      initialValues={account}
      onFinish={async (values) => {
        onSubmit(values);
      }}
    >
      <ProFormText
        name="account_name"
        label={intl.formatMessage({ id: 'plugin.pay.account-name' })}
        width="lg"
      />
      <ProFormRadio.Group
        label={intl.formatMessage({ id: 'plugin.pay.status' })}
        name="status"
        options={[
          {
            value: 1,
            label: intl.formatMessage({ id: 'plugin.pay.status.normal' }),
          },
          {
            value: 0,
            label: intl.formatMessage({ id: 'plugin.pay.status.stop' }),
          },
          {
            value: 2,
            label: intl.formatMessage({ id: 'plugin.pay.status.pause' }),
          },
        ]}
      />
      <ProFormCheckbox
        label={intl.formatMessage({ id: 'plugin.pay.is-default' })}
        name="is_default"
      >
        {intl.formatMessage({ id: 'plugin.pay.is-default' })}
      </ProFormCheckbox>
      <Divider orientation="left">
        <Button
          type="link"
          onClick={() => {
            setShowMore(!showMore);
          }}
        >
          {showMore ? (
            <FormattedMessage id="plugin.pay.show-less" />
          ) : (
            <FormattedMessage id="plugin.pay.show-more" />
          )}
        </Button>
      </Divider>
      <div style={{ display: showMore ? 'block' : 'none' }}>
        <ProFormDigit
          name="health_score"
          label={intl.formatMessage({ id: 'plugin.pay.health-score' })}
          width="lg"
        />
        <ProFormDigit
          name="weight"
          label={intl.formatMessage({ id: 'plugin.pay.weight' })}
          width="lg"
        />
        <ProFormFieldSet
          label={intl.formatMessage({ id: 'plugin.pay.amount-limit' })}
        >
          <ProFormDigit
            name="min_amount"
            label={intl.formatMessage({ id: 'plugin.pay.amount-limit.min' })}
            fieldProps={{
              precision: 2,
            }}
          />
          <ProFormDigit
            name="max_amount"
            label={intl.formatMessage({ id: 'plugin.pay.amount-limit.max' })}
            fieldProps={{
              precision: 2,
            }}
          />
        </ProFormFieldSet>
        <ProFormDigit
          name="daily_count_limit"
          label={intl.formatMessage({ id: 'plugin.pay.daily-count-limit' })}
          width="lg"
          fieldProps={{
            suffix: intl.formatMessage({
              id: 'plugin.pay.daily-count-limit.suffix',
            }),
          }}
        />
        <ProFormDigit
          name="daily_amount_limit"
          label={intl.formatMessage({ id: 'plugin.pay.daily-amount-limit' })}
          fieldProps={{
            precision: 2,
          }}
          width="lg"
        />
        <ProFormDigit
          name="monthly_amount_limit"
          label={intl.formatMessage({ id: 'plugin.pay.monthly-amount-limit' })}
          fieldProps={{
            precision: 2,
          }}
          width="lg"
        />
      </div>
      <ProFormRadio.Group
        label={intl.formatMessage({ id: 'plugin.pay.pay-way' })}
        name="pay_way"
        options={PayWayOptions}
        rules={[
          {
            required: true,
            message: intl.formatMessage({
              id: 'plugin.pay.pay-way.required',
            }),
          },
        ]}
        fieldProps={{
          onChange: (e) => {
            setPayWay(e.target.value);
          },
        }}
      />

      <Divider />
      {payWay !== '' && payWay !== 'offline' && (
        <>
          {(payWay === 'wechat' || payWay === 'weapp') && (
            <>
              <ProFormText
                name={['pay_config', 'app_id']}
                label={intl.formatMessage({
                  id:
                    payWay === 'wechat'
                      ? 'plugin.pay.wechat.wechat.appid'
                      : 'plugin.pay.wechat.weapp.appid',
                })}
                width="lg"
              />
              <ProFormText
                name={['pay_config', 'app_secret']}
                label={intl.formatMessage({
                  id:
                    payWay === 'wechat'
                      ? 'plugin.pay.wechat.wechat.app-secret'
                      : 'plugin.pay.wechat.weapp.app-secret',
                })}
                width="lg"
              />
              <ProFormText
                name={['pay_config', 'account']}
                label={intl.formatMessage({ id: 'plugin.pay.wechat.mchid' })}
                width="lg"
              />
              <ProFormText
                name={['pay_config', 'api_key']}
                label={intl.formatMessage({ id: 'plugin.pay.wechat.apikey' })}
                width="lg"
              />
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.pay.wechat.cert-path',
                })}
              >
                <Upload
                  name="file"
                  className="logo-uploader"
                  showUploadList={false}
                  accept=".crt,.pem"
                  customRequest={async (e) => handleUploadFile('cert_path', e)}
                >
                  <Button type="primary">
                    <FormattedMessage id="plugin.pay.upload" />
                  </Button>
                </Upload>
                {account.pay_config.cert_path && (
                  <div className="upload-file">
                    {account.pay_config.cert_path}
                  </div>
                )}
              </ProFormText>
              <ProFormText
                label={intl.formatMessage({ id: 'plugin.pay.wechat.key-path' })}
              >
                <Upload
                  name="file"
                  className="logo-uploader"
                  showUploadList={false}
                  accept=".crt,.pem"
                  customRequest={async (e) =>
                    handleUploadFile('public_cert_path', e)
                  }
                >
                  <Button type="primary">
                    <FormattedMessage id="plugin.pay.upload" />
                  </Button>
                </Upload>
                {account.pay_config.public_cert_path && (
                  <div className="upload-file">
                    {account.pay_config.public_cert_path}
                  </div>
                )}
              </ProFormText>
            </>
          )}
          {payWay === 'alipay' && (
            <>
              <ProFormText
                name={['pay_config', 'app_id']}
                label={intl.formatMessage({
                  id: 'plugin.pay.alipay.appid',
                })}
                width="lg"
              />
              <ProFormText
                name={['pay_config', 'app_secret']}
                label={intl.formatMessage({
                  id: 'plugin.pay.alipay.private-key',
                })}
                width="lg"
              />
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.pay.alipay.cert-path',
                })}
              >
                <Upload
                  name="file"
                  className="logo-uploader"
                  showUploadList={false}
                  accept=".crt,.pem"
                  customRequest={async (e) => handleUploadFile('cert_path', e)}
                >
                  <Button type="primary">
                    <FormattedMessage id="plugin.pay.upload" />
                  </Button>
                </Upload>
                {account.pay_config.cert_path && (
                  <div className="upload-file">
                    {account.pay_config.cert_path}
                  </div>
                )}
              </ProFormText>
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.pay.alipay.public-cert-path',
                })}
              >
                <Upload
                  name="file"
                  className="logo-uploader"
                  showUploadList={false}
                  accept=".crt,.pem"
                  customRequest={async (e) =>
                    handleUploadFile('public_cert_path', e)
                  }
                >
                  <Button type="primary">
                    <FormattedMessage id="plugin.pay.upload" />
                  </Button>
                </Upload>
                {account.pay_config.public_cert_path && (
                  <div className="upload-file">
                    {account.pay_config.public_cert_path}
                  </div>
                )}
              </ProFormText>
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.pay.alipay.root-cert-path',
                })}
              >
                <Upload
                  name="file"
                  className="logo-uploader"
                  showUploadList={false}
                  accept=".crt,.pem"
                  customRequest={async (e) =>
                    handleUploadFile('root_cert_path', e)
                  }
                >
                  <Button type="primary">
                    <FormattedMessage id="plugin.pay.upload" />
                  </Button>
                </Upload>
                {account.pay_config.root_cert_path && (
                  <div className="upload-file">
                    {account.pay_config.root_cert_path}
                  </div>
                )}
              </ProFormText>
            </>
          )}
          {payWay === 'paypal' && (
            <>
              <ProFormText
                name={['pay_config', 'app_id']}
                label={intl.formatMessage({
                  id: 'plugin.pay.paypal.client-id',
                })}
                width="lg"
              />
              <ProFormText
                name={['pay_config', 'app_secret']}
                label={intl.formatMessage({
                  id: 'plugin.pay.paypal.secret',
                })}
                width="lg"
              />
            </>
          )}
          {payWay === 'stripe' && (
            <>
              <ProFormText
                name={['pay_config', 'app_id']}
                label={intl.formatMessage({
                  id: 'plugin.pay.stripe.api-key',
                })}
                width="lg"
              />
              <ProFormText
                name={['pay_config', 'app_secret']}
                label={intl.formatMessage({
                  id: 'plugin.pay.stripe.public-key',
                })}
                width="lg"
              />
              <ProFormText
                name={['pay_config', 'webhook_id']}
                label={intl.formatMessage({
                  id: 'plugin.pay.stripe.webhook-secret',
                })}
                width="lg"
              />
            </>
          )}
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.pay.sandbox' })}
            name={['pay_config', 'sandbox']}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.pay.sandbox.yes',
                }),
                value: true,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.pay.sandbox.no',
                }),
                value: false,
              },
            ]}
            extra={intl.formatMessage({
              id: 'plugin.pay.sandbox.description',
            })}
          />
        </>
      )}
    </ModalForm>
  ) : null;
};

export default PaymentAccountForm;
