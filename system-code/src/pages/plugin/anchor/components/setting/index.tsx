import React, { useEffect, useRef, useState } from 'react';
import { Button, message, Modal, Space } from 'antd';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import ProForm, {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';
import { pluginGetAnchorSetting, pluginSaveAnchorSetting } from '@/services/plugin/anchor';

const AnchorSetting: React.FC = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const [fetched, setFetched] = useState<boolean>(false);
  const [setting, setSetting] = useState<any>({});

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetAnchorSetting();
    let setting = res.data || null;
    setSetting(setting);
    setFetched(true);
  };

  const handleSaveSetting = async (values: any) => {
    values = Object.assign(setting, values)
    let res = await pluginSaveAnchorSetting(values);

    if (res.code === 0) {
      message.success(res.msg);
      setVisible(false);
    } else {
      message.error(res.msg);
    }
  };

  return (
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      {fetched && (
        <ModalForm
          width={600}
          title={'锚文本设置'}
          visible={visible}
          modalProps={{
            onCancel: () => {
              setVisible(false);
            },
          }}
          initialValues={setting}
          layout="horizontal"
          onFinish={async (values) => {
            handleSaveSetting(values);
          }}
        >
          <ProFormDigit
            name="anchor_density"
            label="锚文本密度"
            extra="例如：每100字替换一个锚文本，就填写100，默认100"
          />
          <ProFormRadio.Group
            name="replace_way"
            label="替换方式"
            options={[
              { label: '自动替换', value: 1 },
              { label: '手动替换', value: 0 },
            ]}
            extra="内容替换锚文本的方式"
          />
          <ProFormRadio.Group
            name="keyword_way"
            label="替换方式"
            options={[
              { label: '自动提取', value: 1 },
              { label: '手动提取', value: 0 },
            ]}
            extra="选择从内容的关键词标签里提取锚文本关键词的方式"
          />
        </ModalForm>
      )}
    </>
  );
};

export default AnchorSetting;
