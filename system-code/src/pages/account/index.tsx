import React from 'react';
import { message, Card } from 'antd';
import { GridContent } from '@ant-design/pro-layout';
import ProForm, { ProFormText } from '@ant-design/pro-form';
import { useModel, useRequest } from 'umi';
import { getAdminInfo, saveAdmin } from '@/services/admin';

import styles from './index.less';

const AccountSetting: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');

  const { data: currentUser, loading } = useRequest(() => {
    return getAdminInfo();
  });

  const fetchUserInfo = async () => {
    const userInfo = await initialState?.fetchUserInfo?.();
    if (userInfo) {
      await setInitialState((s) => ({
        ...s,
        currentUser: userInfo,
      }));
    }
  };

  const handleFinish = async (values: any) => {
    const hide = message.loading('正在提交中', 0);
    saveAdmin(values).then(async (res) => {
      if (res.code === 0) {
        message.success('更新基本信息成功');
        await fetchUserInfo();
      } else {
        message.error(res.msg);
      }
    }).finally(() => {
      hide();
    });
  };
  return (
    <GridContent>
      <Card title="管理员密码修改" bordered={false}>
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
                      children: '更新基本信息',
                    },
                  }}
                  initialValues={currentUser}
                  hideRequiredMark
                >
                  <ProFormText
                    width="md"
                    name="user_name"
                    label="用户名"
                    rules={[
                      {
                        required: true,
                        message: '请输入管理员用户名!',
                      },
                    ]}
                  />
                  <ProFormText.Password
                    width="md"
                    name="old_password"
                    label="当前密码"
                    placeholder="如果想改密码，请输入当前密码"
                  />
                  <ProFormText.Password width="md" name="password" label="新密码" />
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
