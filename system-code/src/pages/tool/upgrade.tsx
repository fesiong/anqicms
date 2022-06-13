import React, { useEffect, useState } from 'react';
import ProForm, {
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProFormRadio,
} from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Button, Card, message, Modal, Upload } from 'antd';
import { checkVersion, getVersion, upgradeVersion } from '@/services/version';

const ToolUpgradeForm: React.FC<any> = (props) => {
  const [setting, setSetting] = useState<any>(null);
  const [newVersion, setNewVersion] = useState<any>(null);
  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await getVersion();
    let setting = res.data || null;
    setSetting(setting);

    checkVersion().then(res => {
      setNewVersion(res.data || null)

      if (res.data?.description) {
        message.info(res.data.description)
      }
    })
  };

  const upgradeSubmit = async (values: any) => {
    Modal.confirm({
      title: '确定要升级到最新版吗？',
      onOk: () => {
        upgradeVersion(values)
      .then((res) => {
        Modal.info({
          content: res.msg
        });
        getSetting();
      })
      .catch((err) => {
        console.log(err);
      });
      }
    })
  };

  return (
    <PageHeaderWrapper>
      <Card>
        {setting && (
          <ProForm submitter={false} title="系统升级">
            <ProFormText name='old_version' fieldProps={{
              value: setting.version,
            }} label="当前版本" width="lg" readonly />
          {newVersion ?
          <div>
            <ProFormText name='version' fieldProps={{
              value: newVersion.version,
            }} label="最新版本" width="lg" readonly />
            <ProFormText name='description' fieldProps={{
              value: newVersion.description,
            }} label="版本说明" width="lg" readonly />
            <div className='mt-normal'>
              <Button onClick={upgradeSubmit}>升级到最新版</Button>
            </div>
          </div>
          :
          <div>
              你的系统已经是最新版。如果不确定，你可以访问 <a href='https://www.kandaoni.com/download' target={'_blank'}>https://www.kandaoni.com/download</a> 获取最新版
            </div>
          }
          </ProForm>
        )}
      </Card>
    </PageHeaderWrapper>
  );
};

export default ToolUpgradeForm;
