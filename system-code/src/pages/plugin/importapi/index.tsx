import React, { useEffect, useState } from 'react';
import { PageHeaderWrapper } from '@ant-design/pro-layout';
import { Alert, Button, Card, message, Modal, Space, Table, Tag, Tooltip } from 'antd';
import { pluginGetImportApiSetting, pluginUpdateApiToken } from '@/services/plugin/importApi';
import CopyToClipboard from 'react-copy-to-clipboard';
import ProCard from '@ant-design/pro-card';
import { history } from 'umi';
import './index.less';

const PluginImportApi: React.FC<any> = (props) => {
  const [tab, setTab] = useState('1');
  const [setting, setSetting] = useState<any>({});

  useEffect(() => {
    getSetting();
    let hash = history.location.hash || '';
    if (hash) {
      hash = hash.replaceAll("#","")
      setTab(hash)
    }
  }, []);

  const getSetting = async () => {
    const res = await pluginGetImportApiSetting();
    let setting = res.data || {};
    setSetting(setting);
  };

  const handleUpdateToken = () => {
    Modal.confirm({
      title: '确定要更新Token吗？',
      content: '更新后，原Token失效，请使用新api地址导入内容。',
      onOk: async () => {
        let res = await pluginUpdateApiToken({});
        message.info(res.msg);
        getSetting();
      }
    })
  }

  const handleCopied = () => {
    message.success('复制成功');
  }

  return (
    <PageHeaderWrapper>
      <Card>
        <Alert
          message={<div>
            <p>通过伪原创、AI写作等第三方平台产生的内容可以对接API导入本系统。</p>
            <div>
              <Space>
                <span>我的Token：</span>
                <Tag>{setting.token}</Tag>
                <Button size='small' onClick={handleUpdateToken}>更新Token</Button>
              </Space>
            </div>
            </div>}
        />
        <div className="mt-normal">
          <ProCard
            tabs={{
              tabPosition: 'left',
              activeKey: tab,
              onChange: (key: string) => {
                setTab(key);
              },
            }}
          >
            <ProCard.TabPane key="1" tab="文档导入接口">
              <div className="import-fields">
                <div className="field-item">
                  <div className="name">接口地址：</div>
                  <div className="value">
                    <CopyToClipboard
                      text={setting.base_url + '/api/import/archive?token=' + setting.token}
                      onCopy={handleCopied}
                    >
                      <Tooltip title='点击复制'>
                        {setting.base_url}/api/import/archive?token={setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">请求方式：</div>
                  <div className="value">POST</div>
                </div>
                <div className="field-item">
                  <div className="name">请求类型：</div>
                  <div className="value">form-data</div>
                </div>
                <div className="field-item">
                  <div className="name">POST表单字段：</div>
                  <div className="value">
                    <Table
                      size="small"
                      pagination={false}
                      columns={[
                        {
                          title: '字段名',
                          dataIndex: 'title',
                          width: 150,
                        },
                        {
                          title: '是否必填',
                          dataIndex: 'required',
                          width: 100,
                          render: (text: number) => <span>{text ? '必填' : '否'}</span>,
                        },
                        {
                          title: '说明',
                          dataIndex: 'remark',
                        },
                      ]}
                      dataSource={[
                        {
                          title: 'id',
                          required: false,
                          remark: '文档ID，默认自动生成',
                        },
                        {
                          title: 'title',
                          required: true,
                          remark: '文档标题',
                        },
                        {
                          title: 'content',
                          required: true,
                          remark: '文档内容',
                        },
                        {
                          title: 'category_id',
                          required: true,
                          remark: '分类ID',
                        },
                        {
                          title: 'keywords',
                          required: false,
                          remark: '文档关键词',
                        },
                        {
                          title: 'description',
                          required: false,
                          remark: '文档简介',
                        },
                        {
                          title: 'url_token',
                          required: false,
                          remark: '自定义URL别名，仅支持数字、英文字母',
                        },
                        {
                          title: 'images[]',
                          required: false,
                          remark: '文章组图，可以设置最多9张图片。',
                        },
                        {
                          title: 'logo',
                          required: false,
                          remark:
                            '文档的缩略图，可以是绝对地址，如: https://www.kandaoni.com/logo.png 或相对地址，如: /logo.png',
                        },
                        {
                          title: 'publish_time',
                          required: false,
                          remark:
                            '格式：2006-01-02 15:04:05  文档的发布时间，可以是未来的时间，如果是未来的时间，则文档会在等到时间到了才正式发布。',
                        },
                        {
                          title: 'tag',
                          required: false,
                          remark: '文档Tag标签，多个tag用英文逗号分隔,例如：aaa,bbb,ccc',
                        },
                        {
                          title: '其他自定义字段',
                          required: false,
                          remark: '如果你还传了其他自定义字段，并且文档表中存在该字段，则也支持。',
                        },
                        {
                          title: 'draft',
                          required: false,
                          remark: '是否存入到草稿，支持的值有：false|true，填写true时，则发布的文档会保存到草稿',
                        },
                        {
                          title: 'cover',
                          required: false,
                          remark: '当相同标题、ID文档存在时是否覆盖，支持的值有：false|true，填写true时，则会覆盖成最新的内容，设置为false时，则会提示错误',
                        },
                      ]}
                      key="title"
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">返回格式：</div>
                  <div className="value">JSON</div>
                </div>
                <div className="field-item">
                  <div className="name">正确结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": 200,   //返回200表示数据正确，其他值均为错误\n    "msg": "发布成功",   //如果有错误，则这里会描述错误的原因\n    "data": {\n        "url":"https://www.kandaoni.com/..." //这里返回文档的url\n    }\n}'
                            }
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">错误结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": -1,   //返回200表示数据正确，其他值均为错误\n    "msg": "Token错误",   //如果有错误，则这里会描述错误的原因\n}'
                            }
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
            <ProCard.TabPane key="3" tab="获取分类接口">
              <div className="import-fields">
                <div className="field-item">
                  <div className="name">接口地址：</div>
                  <div className="value">
                    <CopyToClipboard
                      text={setting.base_url + '/api/import/categories?token=' + setting.token}
                      onCopy={handleCopied}
                    >
                      <Tooltip title='点击复制'>
                        {setting.base_url}/api/import/categories?token={setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">请求方式：</div>
                  <div className="value">POST</div>
                </div>
                <div className="field-item">
                  <div className="name">请求类型：</div>
                  <div className="value">form-data</div>
                </div>
                <div className="field-item">
                  <div className="name">POST表单字段：</div>
                  <div className="value">
                    <Table
                      size="small"
                      pagination={false}
                      columns={[
                        {
                          title: '字段名',
                          dataIndex: 'title',
                          width: 150,
                        },
                        {
                          title: '是否必填',
                          dataIndex: 'required',
                          width: 100,
                          render: (text: number) => <span>{text ? '必填' : '否'}</span>,
                        },
                        {
                          title: '说明',
                          dataIndex: 'remark',
                        },
                      ]}
                      dataSource={[
                        {
                          title: 'module_id',
                          required: true,
                          remark: '要获取的分类类型，填数字，支持的值：1 文章，2 产品',
                        },
                      ]}
                      key="title"
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">返回格式：</div>
                  <div className="value">JSON</div>
                </div>
                <div className="field-item">
                  <div className="name">正确结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": 200,   //返回200表示数据正确，其他值均为错误\n    "msg": "",   //如果有错误，则这里会描述错误的原因\n    "data": {\n'
                            }

                            {'      [\n'}
                            {
                              '        {\n          "id": 1,\n          "parent_id": 0,\n          "title": "新闻大事",\n        },\n'
                            }
                            {
                              '        {\n          "id": 2,\n          "parent_id": 1,\n          "title": "国际新闻",\n        },\n'
                            }
                            {
                              '        {\n          "id": 3,\n          "parent_id": 1,\n          "title": "国内新闻",\n        },\n'
                            }
                            {
                              '        {\n          "id": 4,\n          "parent_id": 0,\n          "title": "案例展示",\n        },\n'
                            }
                            {'      ]\n'}

                            {'    }\n}'}
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">错误结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": -1,   //返回200表示数据正确，其他值均为错误\n    "msg": "Token错误",   //如果有错误，则这里会描述错误的原因\n}'
                            }
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
            <ProCard.TabPane key="4" tab="添加友情链接接口">
              <div className="import-fields">
                <div className="field-item">
                  <div className="name">接口地址：</div>
                  <div className="value">
                    <CopyToClipboard
                      text={setting.base_url + '/api/friendlink/create?token=' + setting.token}
                      onCopy={handleCopied}
                    >
                      <Tooltip title='点击复制'>
                        {setting.base_url}/api/friendlink/create?token={setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">请求方式：</div>
                  <div className="value">POST</div>
                </div>
                <div className="field-item">
                  <div className="name">请求类型：</div>
                  <div className="value">form-data</div>
                </div>
                <div className="field-item">
                  <div className="name">POST表单字段：</div>
                  <div className="value">
                    <Table
                      size="small"
                      pagination={false}
                      columns={[
                        {
                          title: '字段名',
                          dataIndex: 'title',
                          width: 150,
                        },
                        {
                          title: '是否必填',
                          dataIndex: 'required',
                          width: 100,
                          render: (text: number) => <span>{text ? '必填' : '否'}</span>,
                        },
                        {
                          title: '说明',
                          dataIndex: 'remark',
                        },
                      ]}
                      dataSource={[
                        {
                          title: 'title',
                          required: true,
                          remark: '对方关键词',
                        },
                        {
                          title: 'link',
                          required: true,
                          remark: '对方链接',
                        },
                        {
                          title: 'nofollow',
                          required: false,
                          remark: '是否添加nofollow，可选值：0 不添加, 1 添加',
                        },
                        {
                          title: 'back_link',
                          required: false,
                          remark: '对方反链页',
                        },
                        {
                          title: 'my_title',
                          required: false,
                          remark: '我的关键词',
                        },
                        {
                          title: 'my_link',
                          required: false,
                          remark:
                            '我的链接',
                        },
                        {
                          title: 'contact',
                          required: false,
                          remark:
                            '对方联系方式',
                        },
                        {
                          title: 'remark',
                          required: false,
                          remark:
                            '备注信息',
                        },
                      ]}
                      key="title"
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">返回格式：</div>
                  <div className="value">JSON</div>
                </div>
                <div className="field-item">
                  <div className="name">正确结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": 200,   //返回200表示数据正确，其他值均为错误\n    "msg": "链接已保存",   //如果有错误，则这里会描述错误的原因\n}'
                            }
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">错误结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": -1,   //返回200表示数据正确，其他值均为错误\n    "msg": "Token错误",   //如果有错误，则这里会描述错误的原因\n}'
                            }
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
            <ProCard.TabPane key="5" tab="删除友情链接接口">
              <div className="import-fields">
                <div className="field-item">
                  <div className="name">接口地址：</div>
                  <div className="value">
                  <CopyToClipboard
                      text={setting.base_url + '/api/friendlink/delete?token=' + setting.token}
                      onCopy={handleCopied}
                    >
                      <Tooltip title='点击复制'>
                        {setting.base_url}/api/friendlink/delete?token={setting.token}
                      </Tooltip>
                    </CopyToClipboard>
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">请求方式：</div>
                  <div className="value">POST</div>
                </div>
                <div className="field-item">
                  <div className="name">请求类型：</div>
                  <div className="value">form-data</div>
                </div>
                <div className="field-item">
                  <div className="name">POST表单字段：</div>
                  <div className="value">
                    <Table
                      size="small"
                      pagination={false}
                      columns={[
                        {
                          title: '字段名',
                          dataIndex: 'title',
                          width: 150,
                        },
                        {
                          title: '是否必填',
                          dataIndex: 'required',
                          width: 100,
                          render: (text: number) => <span>{text ? '必填' : '否'}</span>,
                        },
                        {
                          title: '说明',
                          dataIndex: 'remark',
                        },
                      ]}
                      dataSource={[
                        {
                          title: 'link',
                          required: true,
                          remark: '对方链接',
                        },
                      ]}
                      key="title"
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">返回格式：</div>
                  <div className="value">JSON</div>
                </div>
                <div className="field-item">
                  <div className="name">正确结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": 200,   //返回200表示数据正确，其他值均为错误\n    "msg": "链接已删除",   //如果有错误，则这里会描述错误的原因\n}'
                            }
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
                <div className="field-item">
                  <div className="name">错误结果示例：</div>
                  <div className="value">
                    <Alert
                      message={
                        <pre>
                          <code>
                            {
                              '{\n    "code": -1,   //返回200表示数据正确，其他值均为错误\n    "msg": "Token错误",   //如果有错误，则这里会描述错误的原因\n}'
                            }
                          </code>
                        </pre>
                      }
                    />
                  </div>
                </div>
              </div>
            </ProCard.TabPane>
          </ProCard>
        </div>
      </Card>
    </PageHeaderWrapper>
  );
};

export default PluginImportApi;
