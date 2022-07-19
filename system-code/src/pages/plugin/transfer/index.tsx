import { Steps, Card, message, Modal, Space, Alert, Divider, Button, Radio } from 'antd';
import React, { useState, useRef, useEffect } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import './index.less';
import { downloadFile } from '@/utils';
import ProForm, { ProFormInstance, ProFormText } from '@ant-design/pro-form';
import { pluginCreateTransferTask, pluginGetTransferTask, pluginStartTransferTask } from '@/services/plugin/transfer';
const { Step } = Steps;

var submitting = false;
var timeingXhr:any = null;

const PluginTransfer: React.FC = () => {
  const formRef = React.createRef<ProFormInstance>();
  const [currentStep, setCurrentStep] = useState<number>(0);
  const [provider, setProvider] = useState<string>('');
  const [task, setTask] = useState<any>({});

  useEffect(() => {
    checkTask();
  }, []);

  const submitProvider = () => {
    if (!provider) {
      message.error("请选择一个网站系统");
      return;
    }
    setCurrentStep(1);
  }

  const downloadProvider = () => {
    downloadFile(
      '/plugin/transfer/download',
      {
        provider: provider,
      },
      provider + '2anqicms.php',
    );
  }

  const submitTask = () => {
    let values = formRef.current?.getFieldsValue();
    values.provider = provider;
    values.name = provider;
    if (!values.token) {
      message.error('请填写通信Token，可以是任意字符');
      return;
    }
    if (values.base_url.indexOf("http") !== 0) {
      message.error('请填写网站地址');
      return;
    }

    if (submitting) {
      return;
    }
    submitting = true;
    const hide = message.loading('正在提交中', 0);

    pluginCreateTransferTask(values).then((res) => {
      if(res.code !== 0) {
        Modal.info({
          title: '通信错误',
          content: res.msg,
        });
      } else {
        message.success('通信成功')
        setTask(res.data);
        setCurrentStep(3);
        checkTask();
      }
    }).finally(() => {
      submitting = false;
      hide();
    });
  }

  const checkTask = () => {
    console.log(timeingXhr)
    if (timeingXhr) {
      return;
    }
    timeingXhr = setInterval(() => {
      pluginGetTransferTask().then(res => {
        if (res.code !== 0) {
          clearInterval(timeingXhr);
          timeingXhr = null;
        } else {
          setTask(res.data);
          if(res.data.status == 1) {
            setCurrentStep(3);
          }
          if (res.data.status != 1) {
            clearInterval(timeingXhr);
            timeingXhr = null;
          }
        }
      }).catch(() => {
        clearInterval(timeingXhr);
        timeingXhr = null;
      });
    }, 2000);
  }

  const startTransfer = () => {
    if(submitting) {
      return;
    }
    submitting = true;
    const hide = message.loading('正在执行中', 0);
    pluginStartTransferTask({}).then(res => {
      if(res.code === 0) {
        checkTask();
      } else {
        message.error(res.msg);
      }
    }).finally(() => {
      submitting = false;
      hide();
    })
  }

  return (
    <PageContainer>
      <Card title='网站内容迁移'>
        <Alert style={{marginBottom: '30px'}} message='目前支持 DedeCMS 和 WordPress 的网站内容迁移到 anqicms 中。'/>
      <Steps progressDot current={currentStep} onChange={setCurrentStep}>
      <Step title="第一步" description="选择需要迁移的网站系统" />
      <Step title="第二步" description="下载通信接口文件" />
      <Step title="第三步" description="填写网站通信信息" />
      <Step title="第四步" description="开始传输网站内容" />
    </Steps>
    <div>
      {currentStep == 0 &&
        <div className='step-content'>
          <Divider>选择需要迁移的网站系统</Divider>
           <Radio.Group
              name="provider"
              options={[
                {
                  value: 'dedecms',
                  label: 'DedeCMS',
                },
                {
                  value: 'wordpress',
                  label: 'WordPress',
                },
              ]}
              value={provider}
              onChange={(e) => {setProvider(e.target.value)}}
            />
          <div className='step-buttons'>
            <Button type='primary' onClick={() => submitProvider()}>下一步</Button>
          </div>
          </div>
      }
      {currentStep == 1 &&
        <div className='step-content'>
          <Divider>下载通信接口文件</Divider>
          <div>
            <Alert message='请把下载的文件上传到你的网站的根目录下。下载并放置到你的网站根目录后，点击下一步继续操作。' />
            <div style={{marginTop: '30px'}}>
            <Button onClick={downloadProvider} type='primary' size='large'>下载「{provider}2anqicms.php」</Button>
            </div>
          </div>
          <div className='step-buttons'>
            <Space size={20}>
            <Button onClick={() => setCurrentStep(currentStep-1)}>上一步</Button>
            <Button type='primary' onClick={() => setCurrentStep(currentStep+1)}>下一步</Button>
            </Space>
          </div>
          </div>
      }
      {currentStep == 2 &&
        <div className='step-content'>
          <Divider>填写网站通信信息</Divider>
          <div>
            <Alert message='每个网站只能配置一个Token，如果你提示错误了，请手动删除网站根目录下的 anqicms.config.php 才能再次配置。' />
            <div style={{marginTop: '30px'}}>
            <ProForm formRef={formRef} initialValues={task} submitter={false}>
            <ProFormText name="base_url" label="网站地址" />
            <ProFormText name="token" label="通信token" placeholder={'可以是任意字符'} />
            </ProForm>
            </div>
          </div>
          <div className='step-buttons'>
            <Space size={20}>
            <Button onClick={() => setCurrentStep(currentStep-1)}>上一步</Button>
            <Button type='primary' onClick={submitTask}>下一步</Button>
            </Space>
          </div>
          </div>
      }
      {currentStep == 3 &&
        <div className='step-content'>
          <Divider>开始传输网站内容</Divider>
          {task &&
          <>
          <div>
            <div style={{marginBottom: '30px'}}><Alert message='迁移过程中，请不要刷新本页面。' /></div>
            <p>需要转移的站点：{task.base_url}</p>
            <p>当前任务状态：{task.status == 2 ? '已完成' : task.status == 1 ? '进行中' : '未开始'}</p>
            {task.status == 1 && <p>当前任务进度：正在迁移 {task.current} {task.last_id > 0 ? '，数据量：' + task.last_id : ''}</p>}
            {task.error_msg && <p>任务出错：{task.error_msg}</p>}
          </div>
          <div className='step-buttons'>
            <Space size={20}>
            {task.status != 1 && <Button onClick={() => setCurrentStep(0)}>重新开始</Button>}
            {task.status == 0 && <Button onClick={() => startTransfer()} type='primary'>开始迁移</Button>}
            </Space>
          </div>
          </>
          }
          </div>
      }
    </div>
      </Card>

    </PageContainer>
  );
};

export default PluginTransfer;
