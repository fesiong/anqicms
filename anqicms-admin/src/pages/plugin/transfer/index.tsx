import {
  pluginCreateTransferTask,
  pluginGetTransferModules,
  pluginGetTransferTask,
  pluginStartTransferTask,
} from '@/services/plugin/transfer';
import { downloadFile } from '@/utils';
import {
  PageContainer,
  ProForm,
  ProFormInstance,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Divider,
  Modal,
  Radio,
  Space,
  Steps,
  message,
} from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';
const { Step } = Steps;

let submitting = false;
let timeingXhr: any = null;

const PluginTransfer: React.FC = () => {
  const formRef = React.createRef<ProFormInstance>();
  const formRef2 = React.createRef<ProFormInstance>();
  const [currentStep, setCurrentStep] = useState<number>(0);
  const [provider, setProvider] = useState<string>('');
  const [task, setTask] = useState<any>({});
  const [modules, setModules] = useState<any[]>([]);
  const [types, setTypes] = useState<string[]>([]);
  const [moduleIds, setModuleIds] = useState<number[]>([]);
  const [selectedTypes, setSelectedTypes] = useState<string[]>([]);
  const [moduleFetched, setModuleFetched] = useState<boolean>(false);
  const intl = useIntl();

  const checkTask = () => {
    if (timeingXhr) {
      return;
    }
    timeingXhr = setInterval(() => {
      pluginGetTransferTask()
        .then((res) => {
          if (res.code !== 0) {
            clearInterval(timeingXhr);
            timeingXhr = null;
          } else {
            setTask(res.data);
            if (res.data.status === 1) {
              setCurrentStep(4);
            }
            if (res.data.status !== 1) {
              clearInterval(timeingXhr);
              timeingXhr = null;
            }
          }
        })
        .catch(() => {
          clearInterval(timeingXhr);
          timeingXhr = null;
        });
    }, 1000);
  };

  useEffect(() => {
    checkTask();
  }, []);

  const submitProvider = () => {
    if (!provider) {
      message.error(
        intl.formatMessage({ id: 'plugin.transfer.provider.required' }),
      );
      return;
    }
    setCurrentStep(1);
  };

  const downloadProvider = () => {
    downloadFile(
      '/plugin/transfer/download',
      {
        provider: provider,
      },
      provider + '2anqicms.php',
    );
  };

  const loadModules = () => {
    pluginGetTransferModules({}).then((res: any) => {
      if (res.code === 0) {
        setModules(res.data.modules || []);
        setTypes(res.data.types || []);
        setSelectedTypes(res.data.types || []);
        let tmpIds = [];
        for (let i in res.data.modules) {
          tmpIds.push(res.data.modules[i].id);
        }
        setModuleIds(tmpIds);
        setModuleFetched(true);
      } else {
        message.error(res.msg);
      }
    });
  };

  const submitTask = () => {
    let values = formRef.current?.getFieldsValue();
    values.provider = provider;
    values.name = provider;
    if (!values.token) {
      message.error(
        intl.formatMessage({ id: 'plugin.transfer.token.required' }),
      );
      return;
    }
    if (values.base_url.indexOf('http') !== 0) {
      message.error(
        intl.formatMessage({ id: 'plugin.transfer.base-url.required' }),
      );
      return;
    }

    if (submitting) {
      return;
    }
    submitting = true;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );

    pluginCreateTransferTask(values)
      .then((res) => {
        if (res.code !== 0) {
          Modal.info({
            title: intl.formatMessage({ id: 'plugin.transfer.signal.error' }),
            content: res.msg,
          });
        } else {
          message.success(
            intl.formatMessage({ id: 'plugin.transfer.signal.success' }),
          );
          setTask(res.data);
          loadModules();
          setCurrentStep(3);
          checkTask();
        }
      })
      .finally(() => {
        submitting = false;
        hide();
      });
  };

  const submitTaskModule = () => {
    setCurrentStep(4);
  };

  const startTransfer = () => {
    if (submitting) {
      return;
    }
    submitting = true;
    const hide = message.loading(
      intl.formatMessage({ id: 'plugin.transfer.transfering' }),
      0,
    );
    pluginStartTransferTask({
      module_ids: moduleIds,
      types: selectedTypes,
    })
      .then((res) => {
        if (res.code === 0) {
          checkTask();
        } else {
          message.error(res.msg);
        }
      })
      .finally(() => {
        submitting = false;
        hide();
      });
  };

  return (
    <PageContainer>
      <Card>
        <Alert
          style={{ marginBottom: '30px' }}
          message={intl.formatMessage({ id: 'plugin.transfer.tips' })}
        />
        <Steps progressDot current={currentStep} onChange={setCurrentStep}>
          <Step
            title={intl.formatMessage({ id: 'plugin.transfer.step1' })}
            description={intl.formatMessage({
              id: 'plugin.transfer.step1.description',
            })}
          />
          <Step
            title={intl.formatMessage({ id: 'plugin.transfer.step2' })}
            description={intl.formatMessage({
              id: 'plugin.transfer.step2.description',
            })}
          />
          <Step
            title={intl.formatMessage({ id: 'plugin.transfer.step3' })}
            description={intl.formatMessage({
              id: 'plugin.transfer.step3.description',
            })}
          />
          <Step
            title={intl.formatMessage({ id: 'plugin.transfer.step4' })}
            description={intl.formatMessage({
              id: 'plugin.transfer.step4.description',
            })}
          />
          <Step
            title={intl.formatMessage({ id: 'plugin.transfer.step5' })}
            description={intl.formatMessage({
              id: 'plugin.transfer.step5.description',
            })}
          />
        </Steps>
        <div>
          {currentStep === 0 && (
            <div className="step-content">
              <Divider>
                <FormattedMessage id="plugin.transfer.step1.description" />
              </Divider>
              <Radio.Group
                name="provider"
                optionType="button"
                options={[
                  {
                    value: 'wordpress',
                    label: 'WordPress',
                  },
                  {
                    value: 'dedecms',
                    label: 'DedeCMS',
                  },
                  {
                    value: 'pbootcms',
                    label: 'PbootCMS',
                  },
                  {
                    value: 'empire',
                    label: 'EmpireCMS',
                  },
                  {
                    value: 'xunruicms',
                    label: 'XunruiCMS',
                  },
                ]}
                value={provider}
                onChange={(e) => {
                  setProvider(e.target.value);
                }}
              />
              <div className="step-buttons">
                <Button type="primary" onClick={() => submitProvider()}>
                  <FormattedMessage id="plugin.transfer.step.next" />
                </Button>
              </div>
            </div>
          )}
          {currentStep === 1 && (
            <div className="step-content">
              <Divider>
                <FormattedMessage id="plugin.transfer.step2.description" />
              </Divider>
              <div>
                <Alert
                  message={intl.formatMessage({
                    id: 'plugin.transfer.step2.tips',
                  })}
                />
                <div style={{ marginTop: '30px' }}>
                  <Button
                    onClick={downloadProvider}
                    type="primary"
                    size="large"
                  >
                    <FormattedMessage id="plugin.transfer.step.download" />「
                    {provider}2anqicms.php」
                  </Button>
                </div>
              </div>
              <div className="step-buttons">
                <Space size={20}>
                  <Button onClick={() => setCurrentStep(currentStep - 1)}>
                    <FormattedMessage id="plugin.transfer.step.prev" />
                  </Button>
                  <Button
                    type="primary"
                    onClick={() => setCurrentStep(currentStep + 1)}
                  >
                    <FormattedMessage id="plugin.transfer.step.next" />
                  </Button>
                </Space>
              </div>
            </div>
          )}
          {currentStep === 2 && (
            <div className="step-content">
              <Divider>
                <FormattedMessage id="plugin.transfer.step3.description" />
              </Divider>
              <div>
                <Alert
                  message={intl.formatMessage({
                    id: 'plugin.transfer.step3.tips',
                  })}
                />
                <div style={{ marginTop: '30px' }}>
                  <ProForm
                    formRef={formRef}
                    initialValues={task}
                    submitter={false}
                  >
                    <ProFormText
                      name="base_url"
                      label={intl.formatMessage({
                        id: 'plugin.transfer.base-url',
                      })}
                      placeholder={intl.formatMessage({
                        id: 'plugin.transfer.base-url.placeholder',
                      })}
                    />
                    <ProFormText
                      name="token"
                      label={intl.formatMessage({
                        id: 'plugin.transfer.token',
                      })}
                      placeholder={intl.formatMessage({
                        id: 'plugin.transfer.token.placeholder',
                      })}
                    />
                    <ProFormText
                      name="target_id"
                      label={intl.formatMessage({
                        id: 'plugin.transfer.site-id',
                      })}
                      placeholder={intl.formatMessage({
                        id: 'plugin.transfer.site-id.placeholder',
                      })}
                    />
                  </ProForm>
                </div>
              </div>
              <div className="step-buttons">
                <Space size={20}>
                  <Button onClick={() => setCurrentStep(currentStep - 1)}>
                    <FormattedMessage id="plugin.transfer.step.prev" />
                  </Button>
                  <Button type="primary" onClick={submitTask}>
                    <FormattedMessage id="plugin.transfer.step.next" />
                  </Button>
                </Space>
              </div>
            </div>
          )}
          {currentStep === 3 && (
            <div className="step-content">
              <Divider>
                <FormattedMessage id="plugin.transfer.step4.description" />
              </Divider>
              <div>
                <Alert
                  message={intl.formatMessage({
                    id: 'plugin.transfer.step4.tips',
                  })}
                />
                <div style={{ marginTop: '30px' }}>
                  {moduleFetched && (
                    <ProForm formRef={formRef2} submitter={false}>
                      <div>
                        <FormattedMessage id="plugin.transfer.types" />
                      </div>
                      <Checkbox.Group
                        name="types"
                        options={types.map((item: any) => ({
                          label: item,
                          value: item,
                        }))}
                        value={selectedTypes}
                        onChange={(e) => {
                          setSelectedTypes(e as string[]);
                        }}
                      />
                      <div>
                        <FormattedMessage id="plugin.transfer.module" />
                      </div>
                      <Checkbox.Group
                        name="module_ids"
                        options={modules.map((item: any) => ({
                          label: item.title,
                          value: item.id,
                        }))}
                        value={moduleIds}
                        onChange={(e) => {
                          setModuleIds(e as number[]);
                        }}
                      />
                    </ProForm>
                  )}
                </div>
              </div>
              <div className="step-buttons">
                <Space size={20}>
                  <Button onClick={() => setCurrentStep(currentStep - 1)}>
                    <FormattedMessage id="plugin.transfer.step.prev" />
                  </Button>
                  <Button type="primary" onClick={submitTaskModule}>
                    <FormattedMessage id="plugin.transfer.step.next" />
                  </Button>
                </Space>
              </div>
            </div>
          )}
          {currentStep === 4 && (
            <div className="step-content">
              <Divider>
                <FormattedMessage id="plugin.transfer.step5.description" />
              </Divider>
              {task && (
                <>
                  <div>
                    <div style={{ marginBottom: '30px' }}>
                      <Alert
                        message={intl.formatMessage({
                          id: 'plugin.transfer.step5.tips',
                        })}
                      />
                    </div>
                    <p>
                      <FormattedMessage id="plugin.transfer.base-url.name" />
                      {task.base_url}
                    </p>
                    <p>
                      <FormattedMessage id="plugin.transfer.status" />
                      {task.status === 2
                        ? intl.formatMessage({
                            id: 'plugin.transfer.status.finished',
                          })
                        : task.status === 1
                        ? intl.formatMessage({
                            id: 'plugin.transfer.status.doing',
                          })
                        : intl.formatMessage({
                            id: 'plugin.transfer.status.wait',
                          })}
                    </p>
                    {task.status === 1 && (
                      <p>
                        <FormattedMessage id="plugin.transfer.current-task" />{' '}
                        {task.current}{' '}
                        {task.last_id > 0
                          ? intl.formatMessage({
                              id: 'plugin.transfer.current-task.count',
                            }) + task.last_id
                          : ''}
                      </p>
                    )}
                    {task.error_msg && (
                      <p>
                        <FormattedMessage id="plugin.transfer.task-error" />
                        {task.error_msg}
                      </p>
                    )}
                  </div>
                  <div className="step-buttons">
                    <Space size={20}>
                      {task.status !== 1 && (
                        <Button onClick={() => setCurrentStep(0)}>
                          <FormattedMessage id="plugin.transfer.restart" />
                        </Button>
                      )}
                      {task.status === 0 && (
                        <Button onClick={() => startTransfer()} type="primary">
                          <FormattedMessage id="plugin.transfer.start" />
                        </Button>
                      )}
                    </Space>
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      </Card>
    </PageContainer>
  );
};

export default PluginTransfer;
