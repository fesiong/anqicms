import React, { useEffect, useState } from 'react';
import ProForm, { ProFormTextArea, ProFormRadio } from '@ant-design/pro-form';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Alert, Card, Col, message, Radio, Row, Space, Tag } from 'antd';
import { pluginGetRewrite, pluginSaveRewrite } from '@/services/plugin/rewrite';

const PluginRewrite: React.FC<any> = (props) => {
  const [rewriteMode, setRewriteMode] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [currentMode, setCurrentMode] = useState<number>(0);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetRewrite();
    let setting = res.data || {};
    setRewriteMode(setting);
    setCurrentMode(setting.mode || 0);
    setFetched(true);
  };

  const onSubmit = async (values: any) => {
    values = Object.assign(rewriteMode, values);
    pluginSaveRewrite(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      });
  };
  return (
    <PageHeaderWrapper>
      <Card>
        <Alert
          message={
            <div>
              <Row>
                <Col span={6}>
                  <h3>方案1：数字模式</h3>
                  <div>
                    <div>文档详情：{'/{module}/{id}.html'}</div>
                    <div>文档列表：{'/{module}/{catid}(/{page})'}</div>
                    <div>模型首页：{'/{module}'}</div>
                    <div>单页详情：{'/{id}.html'}</div>
                    <div>标签列表：{'/tags(/{page})'}</div>
                    <div>标签详情：{'/tag/{id}(/{page})'}</div>
                  </div>
                </Col>
                <Col span={6}>
                  <h3>方案2：命名模式1</h3>
                  <div>
                    <div>文档详情：{'/{module}/{filename}.html'}</div>
                    <div>文档列表：{'/{module}/{catname}(/{page})'}</div>
                    <div>模型首页：{'/{module}'}</div>
                    <div>单页详情：{'/{filename}.html'}</div>
                    <div>标签列表：{'/tags(/{page})'}</div>
                    <div>标签详情：{'/tag/{filename}(/{page})'}</div>
                  </div>
                </Col>
                <Col span={6}>
                  <h3>方案3：命名模式2</h3>
                  <div>
                    <div>文档详情：{'/{catname}/{id}.html'}</div>
                    <div>文档列表：{'/{catname}(/{page})'}</div>
                    <div>模型首页：{'/{module}'}</div>
                    <div>单页详情：{'/{filename}.html'}</div>
                    <div>标签列表：{'/tags(/{page})'}</div>
                    <div>标签详情：{'/tag/{id}(/{page})'}</div>
                  </div>
                </Col>
                <Col span={6}>
                  <h3>方案4：命名模式3</h3>
                  <div>
                    <div>文档详情：{'/{catname}/{filename}.html'}</div>
                    <div>文档列表：{'/{catname}(/{page})'}</div>
                    <div>模型首页：{'/{module}'}</div>
                    <div>单页详情：{'/{filename}.html'}</div>
                    <div>标签列表：{'/tags(/{page})'}</div>
                    <div>标签详情：{'/tag/{filename}(/{page})'}</div>
                  </div>
                </Col>
              </Row>
            </div>
          }
        />
        <div className="mt-normal">
          {fetched && (
            <ProForm onFinish={onSubmit} initialValues={rewriteMode} title="伪静态方案设置">
              <ProForm.Item name="mode" label="选择伪静态方案">
                <Radio.Group
                  onChange={(e) => {
                    setCurrentMode(e.target.value);
                  }}
                >
                  <Space direction="vertical">
                    <Radio value={0}>方案1：数字模式（简单，推荐）</Radio>
                    <Radio value={1}>方案2：命名模式1（英文或拼音）</Radio>
                    <Radio value={2}>方案3：命名模式2（英文或拼音+数字）</Radio>
                    <Radio value={3}>方案4：命名模式3（英文或拼音）</Radio>
                    <Radio value={4}>
                      方案5：自定义模式（高级模式，请谨慎使用，若设置不当，会导致前端页面打不开）
                    </Radio>
                  </Space>
                </Radio.Group>
              </ProForm.Item>
              {currentMode == 4 && (
                <ProFormTextArea
                  name="patten"
                  fieldProps={{ rows: 8 }}
                  label="自定义伪静态规则"
                  width={600}
                />
              )}
            </ProForm>
          )}
        </div>
        <div className="mt-normal">
          <Card size="small" title="自定义伪静态规则说明" bordered={false}>
            <div>
              请复制下面的规则到输入框里修改,一共6行,分别是文档详情、文档列表、模型首页、页面、标签列表、标签详情。===和前面部分不可修改。
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
              变量由花括号包裹 <Tag color="blue">{'{}'}</Tag>,如 <Tag color="blue">{'{id}'}</Tag>
              。可用的变量有:数据ID <Tag color="blue">{'{id}'}</Tag>、数据自定义链接名{' '}
              <Tag color="blue">{'{filename}'}</Tag>、分类自定义链接名{' '}
              <Tag color="blue">{'{catname}'}</Tag>、分类ID <Tag color="blue">{'{catid}'}</Tag>
              、模型表名 <Tag color="blue">{'{module}'}</Tag>
              ,分页页码 <Tag color="blue">{'{page}'}</Tag>
              ,分页需放在小括号内,如: <Tag color="blue">{'(/{page})'}</Tag>
            </p>
            <div>可直接使用的方案1:</div>
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
            <div>可直接使用的方案2:</div>
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
            <div>可直接使用的方案3:</div>
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
            <div>可直接使用的方案4:</div>
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
    </PageHeaderWrapper>
  );
};

export default PluginRewrite;
