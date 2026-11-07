import { brand } from '@/oem/brand';
import {
  anqiRestart,
  checkVersion,
  getVersion,
  upgradeVersion,
} from '@/services';
import {
  PageContainer,
  ProForm,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Modal, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';

let loading = false;

const ToolUpgradeForm: React.FC<any> = () => {
  const [setting, setSetting] = useState<any>(null);
  const [newVersion, setNewVersion] = useState<any>(null);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getVersion();
    let setting = res.data || null;

    setSetting(setting);

    checkVersion().then((res) => {
      setNewVersion(res.data || null);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  // 重启是升级的必做步骤，不是可选项：
  // 升级会在磁盘上把主程序 rename 成 xxx.old，而进程还跑在旧 inode 上。不重启就再点升级，
  // 后端会把它当成备份程序拒绝（并提示先重启），否则就会把备份再备份成 xxx.old.old。
  const restartNow = () => {
    const hide = message.loading(
      intl.formatMessage({ id: 'tool.restarting' }),
      0,
    );
    anqiRestart({})
      .catch(() => {})
      .finally(() => {
        setTimeout(() => {
          hide();
          window.location.reload();
        }, 3000);
      });
  };

  const upgradeSubmit = async () => {
    if (loading) {
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'tool.confirm-upgrade' }),
      onOk: () => {
        loading = true;
        const hide = message.loading(
          intl.formatMessage({ id: 'tool.upgrading' }),
          0,
        );
        upgradeVersion({ version: newVersion.version })
          .then((res: any) => {
            // code !== 0 时后端并没有换掉主程序，此时提示"重启运行新版"是在骗人。
            if (res?.code !== 0) {
              Modal.error({
                title: intl.formatMessage({ id: 'tool.upgrade-failed' }),
                content: res?.msg,
              });
              getSetting();
              return;
            }
            Modal.info({
              title: intl.formatMessage({ id: 'tool.upgrade-done' }),
              content: res.msg,
              okText: intl.formatMessage({ id: 'tool.confirm-restart' }),
              // 不允许绕开：ESC / 点遮罩 / 右上角关闭都会让用户停在旧进程上。
              closable: false,
              maskClosable: false,
              keyboard: false,
              onOk: restartNow,
            });
            getSetting();
          })
          .catch((err) => {
            console.log(err);
          })
          .finally(() => {
            loading = false;
            hide();
          });
      },
    });
  };

  return (
    <PageContainer>
      <Card>
        {setting && (
          <ProForm
            submitter={false}
            title={intl.formatMessage({ id: 'menu.upgrade' })}
          >
            <ProFormText
              name="old_version"
              fieldProps={{
                value: setting.version,
              }}
              label={intl.formatMessage({ id: 'tool.version' })}
              width="lg"
              readonly
            />
            {newVersion ? (
              <div>
                <ProFormText
                  label={intl.formatMessage({ id: 'tool.new-version' })}
                  width="lg"
                  readonly
                >
                  <Space>
                    <div className="text-primary">{newVersion.version}</div>
                    {newVersion.trial && (
                      <Tag color="orange">
                        {intl.formatMessage({
                          id: 'dashboard.soft-info.trial-version',
                        })}
                      </Tag>
                    )}
                  </Space>
                </ProFormText>
                <ProFormText
                  label={intl.formatMessage({ id: 'tool.version.description' })}
                  width="lg"
                  readonly
                >
                  <div
                    className="elem-quote"
                    dangerouslySetInnerHTML={{ __html: newVersion.description }}
                  ></div>
                </ProFormText>
                <div className="mt-normal">
                  <Button type="primary" onClick={upgradeSubmit}>
                    <FormattedMessage id="tool.upgrade-to-new" />
                    <FormattedMessage id="dashboard.soft-info.trial-version" />
                  </Button>
                </div>
              </div>
            ) : (
              <div>
                <FormattedMessage id="tool.version.tips.before" />{' '}
                <a href={`${brand.website}/download`} target={'_blank'}>
                  {`${brand.website}/download`}
                </a>{' '}
                <FormattedMessage id="tool.version.tips.after" />
              </div>
            )}
          </ProForm>
        )}
      </Card>
    </PageContainer>
  );
};

export default ToolUpgradeForm;
