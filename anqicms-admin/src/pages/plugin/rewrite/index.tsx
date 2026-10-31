import { pluginGetRewrite, pluginSaveRewrite } from '@/services/plugin/rewrite';
import {
  PageContainer,
  ProForm,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Card, Col, Radio, Row, Space, message } from 'antd';
import React, { useEffect, useState } from 'react';

const PluginRewrite: React.FC<any> = () => {
  const [rewriteMode, setRewriteMode] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [currentMode, setCurrentMode] = useState<number>(0);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetRewrite();
    let setting = res.data || {};
    setRewriteMode(setting);
    setCurrentMode(setting.mode || 0);
    setFetched(true);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (data: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    let values = Object.assign(rewriteMode, data);
    pluginSaveRewrite(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };
  return (
    <PageContainer>
      <Card>
        <Alert
          message={
            <div>
              <Row>
                <Col sm={6} xs={24}>
                  <h3>
                    <FormattedMessage id="plugin.rewrite.formula1" />
                  </h3>
                  <div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-detail" />
                      {'/{module}/{id}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-list" />
                      {'/{module}/{catid}(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.module-index" />
                      {'/{module}'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.page-detail" />
                      {'/{id}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-list" />
                      {'/tags(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-detail" />
                      {'/tag/{id}(/{page})'}
                    </div>
                  </div>
                </Col>
                <Col sm={6} xs={24}>
                  <h3>
                    <FormattedMessage id="plugin.rewrite.formula2" />
                  </h3>
                  <div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-detail" />
                      {'/{module}/{filename}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-list" />
                      {'/{module}/{catname}(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.module-index" />
                      {'/{module}'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.page-detail" />
                      {'/{filename}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-list" />
                      {'/tags(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-detail" />
                      {'/tag/{filename}(/{page})'}
                    </div>
                  </div>
                </Col>
                <Col sm={6} xs={24}>
                  <h3>
                    <FormattedMessage id="plugin.rewrite.formula3" />
                  </h3>
                  <div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-detail" />
                      {'/{catname}/{id}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-list" />
                      {'/{catname}(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.module-index" />
                      {'/{module}'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.page-detail" />
                      {'/{filename}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-list" />
                      {'/tags(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-detail" />
                      {'/tag/{id}(/{page})'}
                    </div>
                  </div>
                </Col>
                <Col sm={6} xs={24}>
                  <h3>
                    <FormattedMessage id="plugin.rewrite.formula4" />
                  </h3>
                  <div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-detail" />
                      {'/{catname}/{filename}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.archive-list" />
                      {'/{catname}(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.module-index" />
                      {'/{module}'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.page-detail" />
                      {'/{filename}.html'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-list" />
                      {'/tags(/{page})'}
                    </div>
                    <div>
                      <FormattedMessage id="plugin.rewrite.formula.tag-detail" />
                      {'/tag/{filename}(/{page})'}
                    </div>
                  </div>
                </Col>
              </Row>
            </div>
          }
        />
        <div className="mt-normal">
          {fetched && (
            <ProForm
              onFinish={onSubmit}
              initialValues={rewriteMode}
              title={intl.formatMessage({ id: 'plugin.rewrite.setting' })}
            >
              <ProForm.Item
                name="mode"
                label={intl.formatMessage({
                  id: 'plugin.rewrite.setting.select',
                })}
              >
                <Radio.Group
                  onChange={(e) => {
                    setCurrentMode(e.target.value);
                  }}
                >
                  <Space direction="vertical">
                    <Radio value={0}>
                      <FormattedMessage id="plugin.rewrite.formula1" />
                    </Radio>
                    <Radio value={1}>
                      <FormattedMessage id="plugin.rewrite.formula2" />
                    </Radio>
                    <Radio value={2}>
                      <FormattedMessage id="plugin.rewrite.formula3" />
                    </Radio>
                    <Radio value={3}>
                      <FormattedMessage id="plugin.rewrite.formula4" />
                    </Radio>
                    <Radio value={4}>
                      <FormattedMessage id="plugin.rewrite.formula5" />
                    </Radio>
                  </Space>
                </Radio.Group>
              </ProForm.Item>
              {currentMode === 4 && (
                <ProFormTextArea
                  name="patten"
                  fieldProps={{ rows: 8 }}
                  label={intl.formatMessage({
                    id: 'plugin.rewrite.setting.diy',
                  })}
                  width={600}
                />
              )}
            </ProForm>
          )}
        </div>
        <div className="mt-normal">
          <Card
            size="small"
            title={intl.formatMessage({
              id: 'plugin.rewrite.setting.diy.explain',
            })}
            bordered={false}
          >
            <div>
              <div
                dangerouslySetInnerHTML={{
                  __html: intl.formatMessage({
                    id: 'plugin.rewrite.setting.diy.tips',
                  }),
                }}
              ></div>
            </div>
            <Alert
              className="elem-quote"
              message={
                <code>
                  <pre>
                    {'archive===/{module}-{id}.html'}
                    {'\n'}
                    {'category===/{module}-{filename}(-{page})'}
                    {'\n'}
                    {'archiveIndex===/{module}.html'}
                    {'\n'}
                    {'page===/{filename}.html'}
                    {'\n'}
                    {'tagIndex===/tags(-{page})'}
                    {'\n'}
                    {'tag===/tag-{id}(-{page})'}
                  </pre>
                </code>
              }
            />
            <p>
              <FormattedMessage id="plugin.rewrite.variable.tips" />
            </p>
            <div>
              <FormattedMessage id="plugin.rewrite.formula.direct1" />:
            </div>
            <Alert
              className="elem-quote"
              message={
                <code>
                  <pre>
                    {'archive===/{module}-{id}.html'}
                    {'\n'}
                    {'category===/{module}-{filename}(-{page})'}
                    {'\n'}
                    {'archiveIndex===/{module}.html'}
                    {'\n'}
                    {'page===/{filename}.html'}
                    {'\n'}
                    {'tagIndex===/tags(-{page})'}
                    {'\n'}
                    {'tag===/tag-{id}(-{page})'}
                  </pre>
                </code>
              }
            />
            <div>
              <FormattedMessage id="plugin.rewrite.formula.direct2" />:
            </div>
            <Alert
              className="elem-quote"
              message={
                <code>
                  <pre>
                    {'archive===/{catname}/{id}.html'}
                    {'\n'}
                    {'category===/{filename}(-{page})'}
                    {'\n'}
                    {'archiveIndex===/{module}.html'}
                    {'\n'}
                    {'page===/{filename}.html'}
                    {'\n'}
                    {'tagIndex===/tags(-{page})'}
                    {'\n'}
                    {'tag===/tag-{filename}(-{page})'}
                  </pre>
                </code>
              }
            />
            <div>
              <FormattedMessage id="plugin.rewrite.formula.direct3" />:
            </div>
            <Alert
              className="elem-quote"
              message={
                <code>
                  <pre>
                    {'archive===/{module}/{id}.html'}
                    {'\n'}
                    {'category===/{module}/{filename}(-{page})'}
                    {'\n'}
                    {'archiveIndex===/{module}.html'}
                    {'\n'}
                    {'page===/{filename}.html'}
                    {'\n'}
                    {'tagIndex===/tags(-{page})'}
                    {'\n'}
                    {'tag===/tag/{filename}(-{page})'}
                  </pre>
                </code>
              }
            />
            <div>
              <FormattedMessage id="plugin.rewrite.formula.direct4" />:
            </div>
            <Alert
              className="elem-quote"
              message={
                <code>
                  <pre>
                    {'archive===/{module}/{id}.html'}
                    {'\n'}
                    {'category===/{module}/{id}(-{page})'}
                    {'\n'}
                    {'archiveIndex===/{module}.html'}
                    {'\n'}
                    {'page===/{filename}.html'}
                    {'\n'}
                    {'tagIndex===/tags(/{page})'}
                    {'\n'}
                    {'tag===/tag/{id}(/{page})'}
                  </pre>
                </code>
              }
            />
          </Card>
        </div>
      </Card>
    </PageContainer>
  );
};

export default PluginRewrite;
