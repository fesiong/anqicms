import {
  pluginGetAnchorSetting,
  pluginSaveAnchorSetting,
} from '@/services/plugin/anchor';
import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useEffect, useState } from 'react';

export type AnchorProps = {
  children?: React.ReactNode;
};

const AnchorSetting: React.FC<AnchorProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const [fetched, setFetched] = useState<boolean>(false);
  const [setting, setSetting] = useState<any>({});
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetAnchorSetting();
    let setting = res.data || null;
    setSetting(setting);
    setFetched(true);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleSaveSetting = async (values: any) => {
    values = Object.assign(setting, values);
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
          title={intl.formatMessage({ id: 'plugin.anchor.setting' })}
          open={visible}
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
            label={intl.formatMessage({ id: 'plugin.anchor.density' })}
            extra={intl.formatMessage({
              id: 'plugin.anchor.density.description',
            })}
          />
          <ProFormRadio.Group
            name="replace_way"
            label={intl.formatMessage({ id: 'plugin.anchor.replace-way' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.anchor.replace-way.auto',
                }),
                value: 1,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.anchor.replace-way.manual',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.anchor.replace-way.render',
                }),
                value: 2,
              },
            ]}
            extra={intl.formatMessage({
              id: 'plugin.anchor.replace-way.description',
            })}
          />
          <ProFormRadio.Group
            name="no_strong_tag"
            label={intl.formatMessage({ id: 'plugin.anchor.no_strong_tag' })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.anchor.no_strong_tag.no',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.anchor.no_strong_tag.yes',
                }),
                value: 1,
              },
            ]}
            extra={intl.formatMessage({
              id: 'plugin.anchor.no_strong_tag.description',
            })}
          />
          <ProFormRadio.Group
            name="keyword_way"
            label={intl.formatMessage({ id: 'plugin.anchor.extract' })}
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.anchor.extract.auto' }),
                value: 1,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.anchor.extract.manual',
                }),
                value: 0,
              },
            ]}
            extra={intl.formatMessage({
              id: 'plugin.anchor.extract.description',
            })}
          />
        </ModalForm>
      )}
    </>
  );
};

export default AnchorSetting;
