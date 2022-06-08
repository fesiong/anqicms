import React, { useEffect, useState } from 'react';
import ProForm, {
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProFormRadio,
  ProFormDateTimePicker,
} from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Button, Card, Col, Collapse, message, Modal, Row, Upload } from 'antd';
import { uploadAttachment } from '@/services/attachment';
import { PlusOutlined } from '@ant-design/icons';
import { getSettingSystem, saveSettingSystem } from '@/services/setting';
import { useModel, utils } from 'umi';
import AttachmentSelect from '@/components/attachment';

const SettingSystemFrom: React.FC<any> = (props) => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const [setting, setSetting] = useState<any>({});
  const [siteLogo, setSiteLogo] = useState<string>('');
  const [site_close, setSiteClose] = useState<number>(0);
  const [extraFields, setExtraFields] = useState<any[]>([]);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await getSettingSystem();
    let setting = res.data || { system: {}, template_names: [] };
    setSetting(setting);
    setSiteLogo(setting.system?.site_logo || '');
    setSiteClose(setting.system?.site_close || 0);
    setExtraFields(setting.system.extra_fields || []);
  };

  const handleSelectLogo = (row: any) => {
    setSiteLogo(row.logo);
    message.success('上传完成');
  };

  const onSubmit = async (values: any) => {
    values.site_logo = siteLogo;
    saveSettingSystem(values)
      .then(async (res) => {
        message.success(res.msg);
        const system = await initialState?.fetchUserInfo?.();
        if (system) {
          await setInitialState((s) => ({
            ...s,
            system: system,
          }));
        }
      })
      .catch((err) => {
        console.log(err);
      });
  };

  console.log(extraFields)
  return (
    <PageHeaderWrapper>
      <Card>
        {setting.system && (
          <ProForm initialValues={setting.system} onFinish={onSubmit} title="全局设置">
            <ProFormText
              name="site_name"
              label="网站名称"
              width="lg"
              extra="该名称会以后缀形式显示在网站标题上"
              rules={[
                {
                  required: true,
                  message: '请输入网站名称！',
                },
              ]}
            />
            <ProFormText
              name="base_url"
              label="网站地址"
              width="lg"
              extra="指该网站的PC端访问网址，如：https://www.kandaoni.com，用来生成全站的绝对地址"
              rules={[
                {
                  required: true,
                  message: '请输入网站首页地址！',
                },
              ]}
            />
            <ProFormText
              name="mobile_url"
              label="移动端地址"
              width="lg"
              extra="指该网站的手机端访问网址，如：https://m.kandaoni.com，如果模板类型为PC+手机站，需要设置。"
            />
            <ProFormText label="网站LOGO" width="lg" extra="网站LOGO会显示在页头">
              <AttachmentSelect onSelect={ handleSelectLogo } visible={false}>
                <div className="ant-upload-item">
                {siteLogo ? (
                  <img src={siteLogo} style={{ width: '100%' }} />
                ) : (
                  <div className='add'>
                    <PlusOutlined />
                    <div style={{ marginTop: 8 }}>上传</div>
                  </div>
                )}
                </div>
                </AttachmentSelect>
            </ProFormText>
            <ProFormText
              name="site_icp"
              label="备案号码"
              width="lg"
              extra={
                <div>
                  ICP备案查询地址：
                  <a href="https://beian.miit.gov.cn/" target="_blank">
                    beian.miit.gov.cn
                  </a>
                  ，只需填主体备案号即可。没有则不填。
                </div>
              }
            />
            <ProFormTextArea
              name="site_copyright"
              width="lg"
              label="版权信息"
              placeholder="版权信息会显示在页尾"
            />
            <ProFormSelect
              name="language"
              width="lg"
              label="默认语言包"
              request={async(params) => {
                let names = [];
                for (let item of setting.languages) {
                  names.push({label: item, value: item})
                }
                return names
              }}
              extra='前端一些内置的文字，会按语言包的设定来显示'
            />
            <ProFormText
              name="admin_url"
              label="后台地址"
              width="lg"
              fieldProps={{
                suffix: '/system/',
                placeholder: 'http或https开头的域名'
              }}
              extra={
                <div>
                  <div>你可以给后台单独设置独立的域名地址，加强安全性。如：https://admin.kandaoni.com</div>
                  <div>注意：在设置之前，必须先解析域名，并绑定域名，否则会无法访问后台。</div>
                </div>
              }
            />
            <ProFormRadio.Group
              name="site_close"
              label="网站状态"
              extra="是否闭站"
              fieldProps={{
                onChange: (e: any) => {
                  setSiteClose(e.target.value);
                },
              }}
              options={[
                {
                  value: 0,
                  label: '正常',
                },
                {
                  value: 1,
                  label: '闭站',
                },
              ]}
            />
            {site_close == 1 && (
              <ProFormTextArea
                name="site_close_tips"
                label="闭站提示"
                width="lg"
                extra={<div>站点关闭后，将会显示上面的提示。支持html标签</div>}
              />
            )}
            <Collapse>
                <Collapse.Panel className='mb-normal' header="自定义参数" extra={<Button size='small' onClick={(e) => {
                  e.stopPropagation()
                  extraFields.push({name: '', value: '', remark: ''})
                  setExtraFields([].concat(extraFields))
                }}>添加参数</Button>} key="1">
                    {extraFields.map((item: any, index: number) => (
                      <Row key={index} gutter={16}>
                        <Col span={8}>
                        <ProFormText
                            name={['extra_fields', index, 'name']}
                            label='参数名'
                            required={true}
                            width="lg"
                            extra='保存后会转换成驼峰命名，可通过该名称调用'
                          />
                          </Col>
                          <Col span={8}>
                        <ProFormText
                            name={['extra_fields', index, 'value']}
                            label='参数值'
                            required={true}
                            width="lg"
                          />
                          </Col>
                        <Col span={6}>
                        <ProFormText
                            name={['extra_fields', index, 'remark']}
                            label='备注'
                            width="lg"
                          />
                          </Col>
                          <Col span={2}>
                            <Button style={{marginTop: '30px'}} onClick={() => {
                               Modal.confirm({
                                 title: '确定要删除这个参数吗？',
                                 onOk: () => {
                                  extraFields.splice(index, 1)
                                  setExtraFields([].concat(extraFields))
                                 }
                               })
                            }}>删除</Button>
                          </Col>
                      </Row>
                    ))}
                </Collapse.Panel>
              </Collapse>
          </ProForm>
        )}
      </Card>
    </PageHeaderWrapper>
  );
};

export default SettingSystemFrom;
