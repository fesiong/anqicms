import React, { useEffect, useState } from 'react';
import ProForm, {
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Card, message } from 'antd';
import { getSettingSafe, saveSettingSafe } from '@/services';

const SettingSafeFrom: React.FC<any> = (props) => {
  const [setting, setSetting] = useState<any>(null);
  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await getSettingSafe();
    let setting = res.data || null;
    setSetting(setting);
  };

  const onSubmit = async (values: any) => {
    values.captcha = Number(values.captcha);
    values.daily_limit = Number(values.daily_limit);
    values.content_limit = Number(values.content_limit);
    values.interval_limit = Number(values.interval_limit);
    const hide = message.loading('正在提交中', 0);
    saveSettingSafe(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      }).finally(() => {
        hide();
      });
  };

  return (
    <PageHeaderWrapper>
      <Card>
        {setting && (
          <ProForm initialValues={setting} onFinish={onSubmit} title="内容安全设置">
            <ProFormRadio.Group
              name="captcha"
              label="留言评论验证码"
              options={[
                {
                  value: 0,
                  label: '关闭',
                },
                {
                  value: 1,
                  label: '开启',
                },
              ]}
              extra='如需开启验证码，请参考验证码标签使用js调用刷新验证码和提交验证数据'
            />
            <ProFormText name="daily_limit" label="同IP每日提交限制" width="lg" fieldProps={{suffix: '次'}} extra='0表示不限制' />
            <ProFormText name="content_limit" label="提交留言内容至少" width="lg" fieldProps={{suffix: '字'}} extra='0表示不限制' />
            <ProFormText name="interval_limit" label="留言提交间隔" width="lg" fieldProps={{suffix: '秒'}} extra='0表示不限制' />
            <ProFormTextArea name="content_forbidden" label="留言敏感词过滤" width="lg" extra='一行一个，提交的留言、评论内容包含有这些词的将会被拒绝。' />
            <ProFormTextArea name="ip_forbidden" label="限制IP地址" width="lg" extra={<div>一行一个，使用这些IP访问的链接将会被拒绝。ip支持的格式：单个IP: 192.168.0.1，某个IP段: 192.168.0.0/16</div>} />
            <ProFormTextArea name="ua_forbidden" label="限制UserAgent" width="lg" extra='一行一个，使用这些UserAgent访问的链接将会被拒绝。' />
          </ProForm>
        )}
      </Card>
    </PageHeaderWrapper>
  );
};

export default SettingSafeFrom;
