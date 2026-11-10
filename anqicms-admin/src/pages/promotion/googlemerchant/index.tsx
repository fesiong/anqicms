import NewContainer from '@/components/NewContainer';
import {
  pluginGetGoogleMerchantAuthUrl,
  pluginGetGoogleMerchantSetting,
  pluginGoogleMerchantAccountSelect,
  pluginGoogleMerchantSync,
  pluginSaveGoogleMerchantSetting,
} from '@/services';
import { ProFormRadio } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Col,
  Flex,
  Modal,
  Row,
  Select,
  Space,
  message,
} from 'antd';
import React, { useEffect, useState } from 'react';
import '../index.less';
import GoogleMerchantSyncLog from './components/syncLog';
import GoogleMerchantSyncStatus from './components/syncStatus';

const GoogleMerchant: React.FC<any> = () => {
  const [googleSetting, setGoogleSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const [logOpen, setLogOpen] = useState<boolean>(false);
  const [syncStatusOpen, setSyncStatusOpen] = useState<boolean>(false);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetGoogleMerchantSetting();
    let setting = res.data || {};

    setGoogleSetting(setting);
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    // 提交前，需要将数据转换成数组
    let setting = {
      ...googleSetting,
      ...values,
    };

    pluginSaveGoogleMerchantSetting(setting)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const handleChangeOpen = (value: boolean) => {
    let setting = {
      ...googleSetting,
      open: value,
    };
    setGoogleSetting(setting);
    onSubmit(setting);
  };

  const handleConnectGoogleAccount = () => {
    pluginGetGoogleMerchantAuthUrl().then((res) => {
      if (res.data) {
        window.open(res.data.url);
      } else {
        message.error(res.msg);
      }
    });
  };

  const handleChangeMerchantAccount = (value: string) => {
    let setting = {
      ...googleSetting,
      merchant_id: value,
    };
    setGoogleSetting(setting);
  };
  const handleSelectMerchantAccount = () => {
    pluginGoogleMerchantAccountSelect({
      merchant_id: googleSetting.merchant_id,
    }).then((res) => {
      if (res.code === 0) {
        setGoogleSetting(res.data);
        message.success(res.msg);
      } else {
        message.error(res.msg);
      }
    });
  };

  const handleSyncProduct = (full: boolean) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.googlemerchant.sync.confirm',
      }),
      content: intl.formatMessage({
        id: 'plugin.googlemerchant.sync.confirm.description',
      }),
      onOk: () => {
        pluginGoogleMerchantSync({
          full: full,
        }).then((res) => {
          if (res.code === 0) {
            message.success(res.msg);
          } else {
            message.error(res.msg);
          }
        });
      },
    });
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
        title={intl.formatMessage({ id: 'menu.promotion.googlemerchant' })}
      >
        <Row gutter={[16, 32]}>
          <Col lg={13} xs={24}>
            <ProFormRadio.Group
              name={'open'}
              label={intl.formatMessage({
                id: 'plugin.googlemerchant.open.name',
              })}
              fieldProps={{
                value: googleSetting.open,
                onChange: (e) => handleChangeOpen(e.target.value),
              }}
              options={[
                {
                  label: intl.formatMessage({
                    id: 'plugin.googlemerchant.open.false',
                  }),
                  value: false,
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.googlemerchant.open.true',
                  }),
                  value: true,
                },
              ]}
              extra={intl.formatMessage({
                id: 'plugin.googlemerchant.open.description',
              })}
            />
          </Col>
          {googleSetting.open && (
            <>
              <Col lg={13} xs={24}>
                <h4>
                  <FormattedMessage id="plugin.googlemerchant.google-account" />
                </h4>
                <p>
                  <FormattedMessage id="plugin.googlemerchant.google-account.description" />
                </p>
                <Alert
                  type="info"
                  message={
                    <Flex gap={20} align="center">
                      {googleSetting.email ? (
                        <div style={{ flex: 1 }}>
                          <FormattedMessage id="plugin.googlemerchant.google-account.connected" />
                        </div>
                      ) : (
                        <div style={{ flex: 1 }}>
                          <FormattedMessage id="plugin.googlemerchant.google-account.unconnected" />
                        </div>
                      )}
                      <Button
                        type="primary"
                        style={{ flex: 0 }}
                        onClick={() => handleConnectGoogleAccount()}
                      >
                        <FormattedMessage id="plugin.googlemerchant.google-account.connect" />
                      </Button>
                    </Flex>
                  }
                ></Alert>
              </Col>
              <Col lg={13} xs={24}>
                <h4>
                  <FormattedMessage id="plugin.googlemerchant.google-merchant-account" />
                </h4>
                <p>
                  <FormattedMessage id="plugin.googlemerchant.google-merchant-account.description" />
                </p>
                <Alert
                  type="info"
                  message={
                    <Flex gap={20} align="center">
                      <Select
                        options={[
                          {
                            label: intl.formatMessage({
                              id: 'plugin.googlemerchant.google-merchant-account.no-select',
                            }),
                            value: '',
                          },
                        ]}
                        value={googleSetting.merchant_id}
                        onChange={(e) =>
                          handleChangeMerchantAccount(e.target.value)
                        }
                        style={{ flex: 1 }}
                      ></Select>
                      <Button
                        type="primary"
                        style={{ flex: 0 }}
                        onClick={() => handleSelectMerchantAccount()}
                      >
                        <FormattedMessage id="plugin.googlemerchant.google-merchant-account.connect" />
                      </Button>
                    </Flex>
                  }
                ></Alert>
              </Col>
              {/* <Col lg={13} xs={24}>
            <h4>利用Google工具验证网站</h4>
            <p>为了让Google能够验证你的网域，请将 标记中ID部分黏贴到工具中。</p>
            <Alert
              type="info"
              message={
                <Flex gap={20} align="center">
                  <Input placeholder="MT" style={{ flex: 1 }}></Input>
                  <Button type="primary" style={{ flex: 0 }}>
                    提交
                  </Button>
                </Flex>
              }
            ></Alert>
          </Col> */}
            </>
          )}
          <Col lg={13} xs={24}>
            <Space size={16}>
              <Button onClick={() => handleSyncProduct(true)}>
                <FormattedMessage id="plugin.googlemerchant.full-sync" />
              </Button>
              <Button onClick={() => handleSyncProduct(false)}>
                <FormattedMessage id="plugin.googlemerchant.partial-sync" />
              </Button>
              <Button onClick={() => setLogOpen(true)}>
                <FormattedMessage id="plugin.googlemerchant.log" />
              </Button>
              <Button onClick={() => setSyncStatusOpen(true)}>
                <FormattedMessage id="plugin.googlemerchant.sync-status.result" />
              </Button>
            </Space>
          </Col>
        </Row>
        {logOpen && (
          <GoogleMerchantSyncLog
            open={logOpen}
            onOpenChange={(flag) => setLogOpen(flag)}
          />
        )}
        {syncStatusOpen && (
          <GoogleMerchantSyncStatus
            open={syncStatusOpen}
            onOpenChange={(flag) => setSyncStatusOpen(flag)}
          />
        )}
      </Card>
    </NewContainer>
  );
};

export default GoogleMerchant;
