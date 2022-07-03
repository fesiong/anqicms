import React, { useEffect, useState } from 'react';
import ProForm, {
  ProFormText,
} from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Button, Card, Col, Collapse, message, Modal, Row } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { getSettingContact, saveSettingContact } from '@/services/setting';
import AttachmentSelect from '@/components/attachment';

const SettingContactFrom: React.FC<any> = (props) => {
  const [setting, setSetting] = useState<any>(null);
  const [qrcode, setQrcode] = useState<string>('');
  const [extraFields, setExtraFields] = useState<any[]>([]);
  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await getSettingContact();
    let setting = res.data || null;
    setSetting(setting);
    setQrcode(setting?.qrcode || '');
    setExtraFields(setting.extra_fields || []);
  };

  const handleSelectLogo = (row: any) => {
    setQrcode(row.logo);
    message.success('上传完成');
  };

  const onSubmit = async (values: any) => {
    values.qrcode = qrcode;
    const hide = message.loading('正在提交中', 0);
    saveSettingContact(values)
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
          <ProForm initialValues={setting} onFinish={onSubmit} title="联系方式设置">
            <ProFormText name="user_name" label="联系人" width="lg" />
            <ProFormText name="cellphone" label="联系电话" width="lg" />
            <ProFormText name="address" label="联系地址" width="lg" />
            <ProFormText name="email" label="联系邮箱" width="lg" />
            <ProFormText name="wechat" label="微信号" width="lg" />
            <ProFormText label="微信二维码" width="lg" extra="微信二维码">
              <AttachmentSelect onSelect={ handleSelectLogo } visible={false}>
                <div className="ant-upload-item">
                {qrcode ? (
                  <img src={qrcode} style={{ width: '100%' }} />
                ) : (
                  <div className='add'>
                    <PlusOutlined />
                    <div style={{ marginTop: 8 }}>上传</div>
                  </div>
                )}
                </div>
              </AttachmentSelect>
            </ProFormText>

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

export default SettingContactFrom;
