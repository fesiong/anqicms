import { saveAdmin } from '@/services/admin';
import { GridContent, ProForm, ProFormText } from '@ant-design/pro-components';
import { useIntl, useModel } from '@umijs/max';
import { Card, message } from 'antd';
import React, { useEffect, useState } from 'react';

import styles from './index.less';

const AccountSetting: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const [currentUser, setCurrentUser] = useState<any>({});
  const [loading, setLoading] = useState<boolean>(true);
  const intl = useIntl();

  useEffect(() => {
    setCurrentUser(initialState?.currentUser || {});
    setLoading(false);
  }, []);

  const fetchUserInfo = async () => {
    const userInfo = await initialState?.fetchUserInfo?.();
    if (userInfo) {
      await setInitialState((s) => ({
        ...s,
        currentUser: userInfo,
      }));
      setCurrentUser(userInfo);
    }
  };

  const handleFinish = async (data: any) => {
    let values = Object.assign(initialState?.currentUser || {}, data);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveAdmin(values)
      .then(async (res) => {
        if (res.code === 0) {
          message.success(intl.formatMessage({ id: 'account.base.success' }));
          await fetchUserInfo();
        } else {
          message.error(res.msg);
        }
      })
      .finally(() => {
        hide();
      });
  };
  return (
    <GridContent>
      <Card
        title={intl.formatMessage({ id: 'account.base.change-password' })}
        bordered={false}
      >
        <div className={styles.baseView}>
          {loading ? null : (
            <>
              <div className={styles.left}>
                <ProForm
                  layout="vertical"
                  onFinish={handleFinish}
                  submitter={{
                    resetButtonProps: {
                      style: {
                        display: 'none',
                      },
                    },
                    submitButtonProps: {
                      children: intl.formatMessage({
                        id: 'account.base.update',
                      }),
                    },
                  }}
                  initialValues={currentUser}
                  hideRequiredMark
                >
                  <ProFormText
                    width="md"
                    name="user_name"
                    label={intl.formatMessage({ id: 'account.base.username' })}
                    rules={[
                      {
                        required: true,
                        message: intl.formatMessage({
                          id: 'account.base.username.required',
                        }),
                      },
                    ]}
                  />
                  <ProFormText.Password
                    width="md"
                    name="old_password"
                    label={intl.formatMessage({ id: 'account.base.password' })}
                    placeholder={intl.formatMessage({
                      id: 'account.base.password.description',
                    })}
                  />
                  <ProFormText.Password
                    width="md"
                    name="password"
                    label={intl.formatMessage({
                      id: 'account.base.password.new',
                    })}
                  />
                </ProForm>
              </div>
            </>
          )}
        </div>
      </Card>
    </GridContent>
  );
};

export default AccountSetting;
