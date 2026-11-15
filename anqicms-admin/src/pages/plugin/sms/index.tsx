import {
  pluginSmsLogs,
  pluginSmsSetting,
  pluginSmsSettingSave,
  pluginSmsTest,
} from '@/services/plugin/sms';
import {
  PageContainer,
  ProColumns,
  ProForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Select,
  Tag,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import { buildSmsPayload } from './payload';

// 五个业务场景：test 场景复用 login 模板（规格 §7.2），不单独配模板。
const FORM_SCENES = ['login', 'register', 'reset', 'bind_old', 'bind_new'];
const ALL_SCENES = [...FORM_SCENES, 'test'];
const CONFIG_TABS = ['basic', 'aliyun', 'tencent', 'twilio'];

// 常用区号：文本框默认 86，下拉只做快捷填充，号码仍可手填任意区号。
const COUNTRY_CODES = [
  { label: '+86', value: '86' },
  { label: '+852', value: '852' },
  { label: '+853', value: '853' },
  { label: '+886', value: '886' },
  { label: '+1', value: '1' },
  { label: '+81', value: '81' },
  { label: '+82', value: '82' },
  { label: '+44', value: '44' },
  { label: '+65', value: '65' },
  { label: '+61', value: '61' },
  { label: '+49', value: '49' },
  { label: '+33', value: '33' },
];

const PluginSms: React.FC = () => {
  const intl = useIntl();
  const t = (id: string) => intl.formatMessage({ id });
  // 配置类四个 tab 共用同一个 ProForm 实例，提交时取全量 store 值。
  const formRef = useRef<ProFormInstance>();
  const [testForm] = Form.useForm();
  const [activeTabKey, setActiveTabKey] = useState<string>('basic');
  // 已访问过的 tab 常驻挂载（仅切换 display），避免切 tab 卸载表单丢失未保存输入。
  const [visitedTabs, setVisitedTabs] = useState<string[]>(['basic']);
  const [setting, setSetting] = useState<any>({});
  const [testLoading, setTestLoading] = useState<boolean>(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(
    null,
  );

  // 读回配置：非空凭据是 ******** 掩码，直接回填输入框，不清空。
  const loadSetting = async () => {
    try {
      const res = await pluginSmsSetting();
      const data = res.data || {};
      setSetting(data);
      formRef.current?.setFieldsValue(data);
      return data;
    } catch (err: any) {
      message.error(err?.message || String(err));
      return undefined;
    }
  };

  useEffect(() => {
    loadSetting();
  }, []);

  const renderTab = (key: string, node: React.ReactNode) => {
    if (!visitedTabs.includes(key)) {
      return null;
    }
    return (
      <div style={{ display: activeTabKey === key ? 'block' : 'none' }}>
        {node}
      </div>
    );
  };

  const handleFinish = async (values: any) => {
    // 页面按 tab 常驻挂载但未访问过的字段未注册，onFinish 的 values 不含它们；
    // 后端厂商对象是整体替换，必须取全量 store 值再经 null map 护栏。
    const allValues = formRef.current?.getFieldsValue(true) || values;
    const payload = buildSmsPayload(allValues);
    const hide = message.loading(t('plugin.sms.saving'), 0);
    try {
      const res = await pluginSmsSettingSave(payload);
      message.success(res.msg || t('plugin.sms.save.success'));
      // 保存成功后重新拉取，让掩码状态与后端一致。
      await loadSetting();
    } catch (err: any) {
      message.error(err?.message || String(err));
    } finally {
      hide();
    }
  };

  const handleTest = async (values: any) => {
    setTestLoading(true);
    setTestResult(null);
    try {
      const res = await pluginSmsTest({
        phone_country: values.phone_country || '86',
        phone: values.phone,
      });
      setTestResult({ ok: res.code === 0, msg: res.msg || '' });
    } catch (err: any) {
      setTestResult({ ok: false, msg: err?.message || String(err) });
    } finally {
      setTestLoading(false);
    }
  };

  const maskPlaceholder = t('plugin.sms.mask.placeholder');
  const templatePlaceholder = t('plugin.sms.template.placeholder');
  const templateHint = t('plugin.sms.template.empty.hint');

  const notActiveAlert = (provider: string) =>
    setting.provider !== provider ? (
      <Alert
        type="info"
        showIcon
        message={t('plugin.sms.not_active')}
        style={{ marginBottom: 16 }}
      />
    ) : null;

  const renderDigits = () => (
    <>
      <ProFormDigit
        name="code_length"
        label={t('plugin.sms.code_length')}
        fieldProps={{ min: 4, max: 8, precision: 0 }}
      />
      <ProFormDigit
        name="code_ttl"
        label={t('plugin.sms.code_ttl')}
        fieldProps={{ min: 1, precision: 0 }}
      />
      <ProFormDigit
        name="resend_interval"
        label={t('plugin.sms.resend_interval')}
        fieldProps={{ min: 0, precision: 0 }}
      />
      <ProFormDigit
        name="max_verify_attempts"
        label={t('plugin.sms.max_verify_attempts')}
        fieldProps={{ min: 1, precision: 0 }}
      />
      <ProFormDigit
        name="daily_limit_per_phone"
        label={t('plugin.sms.daily_limit_per_phone')}
        fieldProps={{ min: 0, precision: 0 }}
      />
      <ProFormDigit
        name="hourly_limit_per_ip"
        label={t('plugin.sms.hourly_limit_per_ip')}
        fieldProps={{ min: 0, precision: 0 }}
      />
      <ProFormDigit
        name="daily_limit_per_ip"
        label={t('plugin.sms.daily_limit_per_ip')}
        fieldProps={{ min: 0, precision: 0 }}
      />
      <ProFormDigit
        name="log_retention_days"
        label={t('plugin.sms.log_retention_days')}
        fieldProps={{ min: 0, precision: 0 }}
      />
    </>
  );

  const renderBasic = () => (
    <>
      <ProFormRadio.Group
        name="provider"
        label={t('plugin.sms.provider')}
        options={[
          { label: t('plugin.sms.provider.off'), value: '' },
          { label: t('plugin.sms.provider.aliyun'), value: 'aliyun' },
          { label: t('plugin.sms.provider.tencent'), value: 'tencent' },
          { label: t('plugin.sms.provider.twilio'), value: 'twilio' },
        ]}
      />
      <ProFormCheckbox.Group
        name="scenes"
        label={t('plugin.sms.scenes')}
        extra={t('plugin.sms.scenes.hint')}
        options={FORM_SCENES.map((scene) => ({
          label: t(`plugin.sms.scene.${scene}`),
          value: scene,
        }))}
      />
      {renderDigits()}
    </>
  );

  const renderAliyunRegion = (region: 'domestic' | 'international') => (
    <Card
      key={region}
      type="inner"
      size="small"
      title={t(`plugin.sms.region.${region}`)}
      style={{ marginBottom: 16 }}
    >
      <ProFormText
        name={['aliyun', region, 'access_key_id']}
        label={t('plugin.sms.access_key_id')}
        placeholder={maskPlaceholder}
      />
      <ProFormText
        name={['aliyun', region, 'access_key_secret']}
        label={t('plugin.sms.access_key_secret')}
        placeholder={maskPlaceholder}
      />
      <ProFormText
        name={['aliyun', region, 'endpoint']}
        label={t('plugin.sms.endpoint')}
        extra={t('plugin.sms.endpoint.hint')}
      />
      <ProFormText
        name={['aliyun', region, 'sign_name']}
        label={t('plugin.sms.sign_name')}
      />
      <ProFormText
        name={['aliyun', region, 'param_name']}
        label={t('plugin.sms.param_name')}
        extra={t('plugin.sms.param_name.hint')}
      />
      <Card type="inner" size="small" title={t('plugin.sms.templates')}>
        {FORM_SCENES.map((scene) => (
          <ProFormText
            key={scene}
            name={['aliyun', region, 'templates', scene]}
            label={t(`plugin.sms.scene.${scene}`)}
            placeholder={templatePlaceholder}
          />
        ))}
      </Card>
    </Card>
  );

  const renderAliyun = () => (
    <>
      {notActiveAlert('aliyun')}
      <Alert
        type="info"
        showIcon
        message={templateHint}
        style={{ marginBottom: 16 }}
      />
      {renderAliyunRegion('domestic')}
      {renderAliyunRegion('international')}
    </>
  );

  const renderTencentRegion = (region: 'domestic' | 'international') => (
    <Card
      key={region}
      type="inner"
      size="small"
      title={t(`plugin.sms.region.${region}`)}
      style={{ marginBottom: 16 }}
    >
      <ProFormText
        name={['tencent', region, 'secret_id']}
        label={t('plugin.sms.secret_id')}
        placeholder={maskPlaceholder}
      />
      <ProFormText
        name={['tencent', region, 'secret_key']}
        label={t('plugin.sms.secret_key')}
        placeholder={maskPlaceholder}
      />
      <ProFormText
        name={['tencent', region, 'sms_sdk_app_id']}
        label={t('plugin.sms.sms_sdk_app_id')}
      />
      <ProFormText
        name={['tencent', region, 'region']}
        label={t('plugin.sms.region')}
        extra={t('plugin.sms.region.hint')}
      />
      <ProFormText
        name={['tencent', region, 'sign_name']}
        label={t('plugin.sms.sign_name')}
      />
      <Card type="inner" size="small" title={t('plugin.sms.templates')}>
        {FORM_SCENES.map((scene) => (
          <ProFormText
            key={scene}
            name={['tencent', region, 'templates', scene]}
            label={t(`plugin.sms.scene.${scene}`)}
            placeholder={templatePlaceholder}
          />
        ))}
      </Card>
    </Card>
  );

  const renderTencent = () => (
    <>
      {notActiveAlert('tencent')}
      <Alert
        type="info"
        showIcon
        message={templateHint}
        style={{ marginBottom: 16 }}
      />
      {renderTencentRegion('domestic')}
      {renderTencentRegion('international')}
    </>
  );

  // Twilio 只有国际 profile（config/sms.go 决策 17），且只支持国际号码。
  const renderTwilio = () => (
    <>
      {notActiveAlert('twilio')}
      <Alert
        type="warning"
        showIcon
        message={t('plugin.sms.twilio.intl_only')}
        style={{ marginBottom: 16 }}
      />
      <Alert
        type="info"
        showIcon
        message={templateHint}
        style={{ marginBottom: 16 }}
      />
      <Card
        type="inner"
        size="small"
        title={t('plugin.sms.region.international')}
      >
        <ProFormText
          name={['twilio', 'international', 'account_sid']}
          label={t('plugin.sms.account_sid')}
          placeholder={maskPlaceholder}
        />
        <ProFormText
          name={['twilio', 'international', 'auth_token']}
          label={t('plugin.sms.auth_token')}
          placeholder={maskPlaceholder}
        />
        <ProFormText
          name={['twilio', 'international', 'api_key_sid']}
          label={t('plugin.sms.api_key_sid')}
          placeholder={maskPlaceholder}
        />
        <ProFormText
          name={['twilio', 'international', 'api_key_secret']}
          label={t('plugin.sms.api_key_secret')}
          placeholder={maskPlaceholder}
        />
        <ProFormText
          name={['twilio', 'international', 'from']}
          label={t('plugin.sms.from')}
        />
        <ProFormText
          name={['twilio', 'international', 'messaging_service_sid']}
          label={t('plugin.sms.messaging_service_sid')}
        />
        <Card
          type="inner"
          size="small"
          title={t('plugin.sms.texts')}
          extra={t('plugin.sms.texts.hint')}
        >
          {FORM_SCENES.map((scene) => (
            <ProFormTextArea
              key={scene}
              name={['twilio', 'international', 'texts', scene]}
              label={t(`plugin.sms.scene.${scene}`)}
              placeholder={templatePlaceholder}
              fieldProps={{ rows: 2 }}
            />
          ))}
        </Card>
      </Card>
    </>
  );

  const renderTest = () => (
    <>
      <Alert
        type="info"
        showIcon
        message={t('plugin.sms.test.hint')}
        style={{ marginBottom: 16 }}
      />
      <Form
        form={testForm}
        layout="vertical"
        onFinish={handleTest}
        style={{ maxWidth: 520 }}
      >
        <Form.Item
          name="phone_country"
          label={t('plugin.sms.test.phone_country')}
          initialValue="86"
          extra={t('plugin.sms.test.phone_country.hint')}
        >
          <Input placeholder="86" />
        </Form.Item>
        <Form.Item label={t('plugin.sms.test.common_codes')}>
          <Select
            placeholder={t('plugin.sms.test.common_codes')}
            options={COUNTRY_CODES}
            onChange={(value) =>
              testForm.setFieldsValue({ phone_country: value })
            }
            style={{ width: 220 }}
            allowClear
          />
        </Form.Item>
        <Form.Item
          name="phone"
          label={t('plugin.sms.test.phone')}
          rules={[
            { required: true, message: t('plugin.sms.test.phone.required') },
          ]}
        >
          <Input placeholder={t('plugin.sms.test.phone')} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={testLoading}>
            {t('plugin.sms.test.send')}
          </Button>
        </Form.Item>
      </Form>
      {testResult && (
        <Alert
          type={testResult.ok ? 'success' : 'error'}
          showIcon
          message={
            testResult.ok
              ? t('plugin.sms.test.success')
              : t('plugin.sms.test.failed')
          }
          description={testResult.msg || undefined}
          style={{ marginTop: 16 }}
        />
      )}
    </>
  );

  const columns: ProColumns<any>[] = [
    {
      title: t('plugin.sms.logs.time'),
      dataIndex: 'created_time',
      width: 170,
      hideInSearch: true,
      render: (_, record) =>
        dayjs((record.created_time || 0) * 1000).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: t('plugin.sms.logs.phone'),
      dataIndex: 'phone',
      width: 160,
      render: (_, record) => `+${record.phone_country || '86'} ${record.phone || ''}`,
    },
    {
      title: t('plugin.sms.logs.scene'),
      dataIndex: 'scene',
      width: 110,
      valueType: 'select',
      valueEnum: ALL_SCENES.reduce((acc: Record<string, string>, scene) => {
        acc[scene] = t(`plugin.sms.scene.${scene}`);
        return acc;
      }, {}),
    },
    {
      title: t('plugin.sms.logs.provider'),
      dataIndex: 'provider',
      width: 110,
      valueType: 'select',
      valueEnum: {
        aliyun: t('plugin.sms.provider.aliyun'),
        tencent: t('plugin.sms.provider.tencent'),
        twilio: t('plugin.sms.provider.twilio'),
      },
    },
    {
      title: t('plugin.sms.logs.status'),
      dataIndex: 'status',
      width: 100,
      valueType: 'select',
      valueEnum: {
        '1': t('plugin.sms.logs.status.success'),
        '0': t('plugin.sms.logs.status.pending'),
        '-1': t('plugin.sms.logs.status.failed'),
      },
      render: (_, record) => {
        const status = Number(record.status);
        if (status === 1) {
          return <Tag color="green">{t('plugin.sms.logs.status.success')}</Tag>;
        }
        if (status === -1) {
          return <Tag color="red">{t('plugin.sms.logs.status.failed')}</Tag>;
        }
        return <Tag>{t('plugin.sms.logs.status.pending')}</Tag>;
      },
    },
    {
      title: t('plugin.sms.logs.template_id'),
      dataIndex: 'template_id',
      hideInSearch: true,
      ellipsis: true,
      tooltip: t('plugin.sms.logs.template_id.hint'),
    },
  ];

  const renderLogs = () => (
    <ProTable<any>
      rowKey="id"
      search={{ labelWidth: 'auto' }}
      pagination={{ pageSize: 20, showSizeChanger: false }}
      columns={columns}
      request={async (params) => {
        const res = await pluginSmsLogs(params);
        return {
          data: res.data || [],
          total: res.total || 0,
          success: res.code === 0,
        };
      }}
      // 失败行展开显示 err_code / err_msg 等排障信息（R115 展示口径）。
      expandable={{
        rowExpandable: (record) => Number(record.status) === -1,
        expandedRowRender: (record) => (
          <div>
            <p>
              <b>{t('plugin.sms.logs.err_code')}：</b>
              {record.err_code || '-'}
            </p>
            <p>
              <b>{t('plugin.sms.logs.err_msg')}：</b>
              {record.err_msg || '-'}
            </p>
            <p>
              <b>{t('plugin.sms.logs.biz_id')}：</b>
              {record.biz_id || '-'}
            </p>
            <p>
              <b>{t('plugin.sms.logs.retryable')}：</b>
              {record.retryable ? t('plugin.sms.yes') : t('plugin.sms.no')}
            </p>
            <p>
              <b>{t('plugin.sms.logs.cost_ms')}：</b>
              {record.cost_ms ?? '-'}
            </p>
          </div>
        ),
      }}
    />
  );

  return (
    <PageContainer>
      <Card
        activeTabKey={activeTabKey}
        onTabChange={(tabKey) => {
          setActiveTabKey(tabKey);
          setVisitedTabs((prev) =>
            prev.includes(tabKey) ? prev : [...prev, tabKey],
          );
        }}
        tabList={CONFIG_TABS.concat(['test', 'logs']).map((key) => ({
          key,
          label: t(`plugin.sms.tab.${key}`),
        }))}
      >
        {/* 配置类 tab 共用同一个 ProForm：常驻挂载，切 tab 只切 display。
            保存/重置按钮只在配置类 tab 显示。 */}
        <ProForm
          formRef={formRef}
          layout="horizontal"
          submitter={{
            searchConfig: {
              submitText: t('plugin.sms.save'),
              resetText: t('plugin.sms.reset'),
            },
            render: (_props, doms) =>
              CONFIG_TABS.includes(activeTabKey) ? doms : [],
          }}
          onFinish={handleFinish}
        >
          {renderTab('basic', renderBasic())}
          {renderTab('aliyun', renderAliyun())}
          {renderTab('tencent', renderTencent())}
          {renderTab('twilio', renderTwilio())}
        </ProForm>
        {renderTab('test', renderTest())}
        {renderTab('logs', renderLogs())}
      </Card>
    </PageContainer>
  );
};

export default PluginSms;
