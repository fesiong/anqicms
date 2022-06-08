import React, { useState } from 'react';
import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-form';
import './index.less';
import { Button, Input, message, Space, Tag } from 'antd';
import { getCollectorSetting, saveCollectorSetting } from '@/services/collector';
import { getCategories } from '@/services/category';

export type CollectorSettingProps = {
  onCancel: (flag?: boolean) => void;
};

class CollectorSetting extends React.Component<CollectorSettingProps> {
  state: { [key: string]: any } = {
    visible: false,
    fetched: false,
    setting: {},
    tmpInput: {},
  };

  componentDidMount() {
    getCollectorSetting().then((res) => {
      let setting = res.data;
      if (!setting.title_exclude) {
        setting.title_exclude = [];
      }
      if (!setting.title_exclude_prefix) {
        setting.title_exclude_prefix = [];
      }
      if (!setting.title_exclude_suffix) {
        setting.title_exclude_suffix = [];
      }
      if (!setting.content_exclude_line) {
        setting.content_exclude_line = [];
      }
      if (!setting.content_exclude) {
        setting.content_exclude = [];
      }
      if (!setting.link_exclude) {
        setting.link_exclude = [];
      }
      if (!setting.content_replace) {
        setting.content_replace = [];
      }
      this.setState({
        setting: setting,
        fetched: true,
      });
    });
  }

  handleSetVisible = (visible: boolean) => {
    this.setState({
      visible,
    });
  };

  handleSubmit = async (values: any) => {
    const { setting } = this.state;
    values = Object.assign(setting, values);
    saveCollectorSetting(values).then((res) => {
      message.info(res.msg);
      this.handleSetVisible(false);
      this.props.onCancel();
    });
  };

  handleRemove = (field: string, index: number) => {
    const { setting } = this.state;
    setting[field].splice(index, 1);
    this.setState({
      setting,
    });
  };

  handleChangeTmpInput = (field: string, e: any) => {
    const { tmpInput } = this.state;
    tmpInput[field] = e.target.value;
    this.setState({
      tmpInput,
    });
  };

  handleAddField = (field: string, e: any) => {
    const { tmpInput, setting } = this.state;
    if (field == 'content_replace') {
      if (!tmpInput['from'] || tmpInput['from'] == tmpInput['to']) {
        return;
      }
      let exists = false;
      for (let item of setting[field]) {
        if (item.from == tmpInput['from']) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        setting[field].push({
          from: tmpInput['from'],
          to: tmpInput['to'],
        });

        tmpInput['from'] = '';
        tmpInput['to'] = '';
      }
    } else {
      setting[field].push(tmpInput[field]);
      tmpInput[field] = '';
    }
    this.setState({
      tmpInput,
      setting,
    });
  };

  render() {
    const { visible, fetched, setting, tmpInput } = this.state;

    return (
      <>
        <div
          onClick={() => {
            this.handleSetVisible(!visible);
          }}
        >
          {this.props.children}
        </div>
        {fetched && (
          <ModalForm
            width={800}
            title={'采集和伪原创设置'}
            initialValues={setting}
            visible={visible}
            //layout="horizontal"
            onVisibleChange={(flag) => {
              this.handleSetVisible(flag);
              if (!flag) {
                this.props.onCancel(flag);
              }
            }}
            onFinish={async (values) => {
              this.handleSubmit(values);
            }}
          >
            <ProFormDigit
              name="title_min_length"
              label="标题最少字数"
              placeholder="默认10个字"
              extra="采集文章的时候，标题字数少于指定的字数，则不会采集"
            />
            <ProFormDigit
              name="content_min_length"
              label="内容最少字数"
              placeholder="默认400个字"
              extra="采集文章的时候，文章内容字数少于指定的字数，则不会采集"
            />
            <ProFormRadio.Group
              name="auto_dig_keyword"
              label="关键词自动拓词"
              options={[
                { label: '否', value: false },
                { label: '自动', value: true },
              ]}
            />
            <ProFormRadio.Group
              name="auto_pseudo"
              label="是否伪原创"
              options={[
                { label: '否', value: false },
                { label: '进行伪原创', value: true },
              ]}
            />
            {/* <ProFormDigit
              name="pseudo_api"
              label="伪原创接口地址"
              extra={
                <div>
                  目前支持搜外管家伪原创接口，接口详情：
                  <a href="https://guanjia.seowhy.com/write/original/api" target={'_blank'}>
                    https://guanjia.seowhy.com/write/original/api
                  </a>
                </div>
              }
            /> */}
            <ProFormDigit
              name="start_hour"
              label="每天开始时间"
              placeholder="默认8点开始"
              extra="请填写0-23的数字"
            />
            <ProFormDigit
              name="end_hour"
              label="每天结束时间"
              placeholder="默认22点结束"
              extra="请填写0-23的数字，0表示全天都发布"
            />
            <ProFormDigit
              name="daily_limit"
              label="每日采集限额"
              placeholder="默认1000"
              extra="每日最大发布文章量，最大不能超过10000，这是一个约数，并不一定能发布到这个数量"
            />
            <ProFormSelect
              label="默认发布文章分类"
              name="category_id"
              extra="如果关键词没设置分类，则采集到的文章默认会被归类到这个分类下"
              request={async () => {
                let res = await getCategories({ type: 1 });
                return res.data || [];
              }}
              fieldProps={{
                fieldNames: {
                  label: 'title',
                  value: 'id',
                },
                optionItemRender(item) {
                  return <div dangerouslySetInnerHTML={{ __html: item.spacer + item.title }}></div>;
                },
              }}
            />
            <ProFormText
              label="标题排除词"
              fieldProps={{
                value: tmpInput.title_exclude || '',
                onChange: this.handleChangeTmpInput.bind(this, 'title_exclude'),
                onPressEnter: this.handleAddField.bind(this, 'title_exclude'),
                suffix: <a onClick={this.handleAddField.bind(this, 'title_exclude')}>按回车添加</a>,
              }}
              extra={
                <div>
                  <div className="text-muted">采集文章的时候，标题出现这些关键词，则不会采集</div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.title_exclude?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(this, 'title_exclude', index)}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label="标题开头排除词"
              fieldProps={{
                value: tmpInput.title_exclude_prefix || '',
                onChange: this.handleChangeTmpInput.bind(this, 'title_exclude_prefix'),
                onPressEnter: this.handleAddField.bind(this, 'title_exclude_prefix'),
                suffix: (
                  <a onClick={this.handleAddField.bind(this, 'title_exclude_prefix')}>按回车添加</a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    采集文章的时候，标题开头出现这些关键词，则不会采集
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.title_exclude_prefix?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(this, 'title_exclude_prefix', index)}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label="标题结尾排除词"
              fieldProps={{
                value: tmpInput.title_exclude_suffix || '',
                onChange: this.handleChangeTmpInput.bind(this, 'title_exclude_suffix'),
                onPressEnter: this.handleAddField.bind(this, 'title_exclude_suffix'),
                suffix: (
                  <a onClick={this.handleAddField.bind(this, 'title_exclude_suffix')}>按回车添加</a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    采集文章的时候，标题结尾出现这些关键词，则不会采集
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.title_exclude_suffix?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(this, 'title_exclude_suffix', index)}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label="内容忽略行"
              fieldProps={{
                value: tmpInput.content_exclude_line || '',
                onChange: this.handleChangeTmpInput.bind(this, 'content_exclude_line'),
                onPressEnter: this.handleAddField.bind(this, 'content_exclude_line'),
                suffix: (
                  <a onClick={this.handleAddField.bind(this, 'content_exclude_line')}>按回车添加</a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    采集文章的时候，内容出现这些词的那一行，将会被移除
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.content_exclude_line?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(this, 'content_exclude_line', index)}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label="链接忽略"
              fieldProps={{
                value: tmpInput.link_exclude || '',
                onChange: this.handleChangeTmpInput.bind(this, 'link_exclude'),
                onPressEnter: this.handleAddField.bind(this, 'link_exclude'),
                suffix: <a onClick={this.handleAddField.bind(this, 'link_exclude')}>按回车添加</a>,
              }}
              extra={
                <div>
                  <div className="text-muted">采集文章的时候，链接出现这些关键词的，则不会采集</div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.link_exclude?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(this, 'link_exclude', index)}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label="内容排除"
              fieldProps={{
                value: tmpInput.content_exclude || '',
                onChange: this.handleChangeTmpInput.bind(this, 'content_exclude'),
                onPressEnter: this.handleAddField.bind(this, 'content_exclude'),
                suffix: (
                  <a onClick={this.handleAddField.bind(this, 'content_exclude')}>按回车添加</a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">采集文章的时候，内容出现这些词，则整篇文章都丢弃</div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.content_exclude?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(this, 'content_exclude', index)}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label="内容替换"
              extra={
                <div>
                  <div className="text-muted">
                    <p>编辑需要替换的关键词对，会在发布文章的时候自动执行替换。</p>
                    <p>
                      替换规则支持正则表达式，如果你对正则表达式熟悉，并且通过普通文本无法达成替换需求的，可以尝试使用正则表达式规则来完成替换。
                    </p>
                    <p>
                      正则表达式规则为：由 <Tag>{'{'}</Tag>开始，并以 <Tag>{'}'}</Tag>
                      结束，中间书写规则代码，如{' '}
                      <Tag>
                        {'{'}[0-9]+{'}'}
                      </Tag>{' '}
                      代表匹配连续的数字。
                    </p>
                    <p>
                      内置部分规则，可以快速使用，已内置的有：
                      <Tag>
                        {'{'}邮箱地址{'}'}
                      </Tag>
                      、
                      <Tag>
                        {'{'}日期{'}'}
                      </Tag>
                      、
                      <Tag>
                        {'{'}时间{'}'}
                      </Tag>
                      、
                      <Tag>
                        {'{'}电话号码{'}'}
                      </Tag>
                      、
                      <Tag>
                        {'{'}QQ号{'}'}
                      </Tag>
                      、
                      <Tag>
                        {'{'}微信号{'}'}
                      </Tag>
                      、
                      <Tag>
                        {'{'}网址{'}'}
                      </Tag>
                    </p>
                    <div>
                      <span className="text-red">*</span>{' '}
                      注意：正则表达式规则书写不当很容易造成错误的替换效果，如微信号规则，会同时影响到邮箱地址、网址的完整性。请谨慎使用。
                    </div>
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.content_replace?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag.from}</span>
                          <span className="divide">替换为</span>
                          <span className="value">{tag.to || '空'}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(this, 'content_replace', index)}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            >
              <Input.Group compact>
                <Input
                  style={{ width: '40%' }}
                  value={tmpInput.from || ''}
                  onChange={this.handleChangeTmpInput.bind(this, 'from')}
                  onPressEnter={this.handleAddField.bind(this, 'content_replace')}
                />
                <span className="input-divide">替换为</span>
                <Input
                  style={{ width: '50%' }}
                  value={tmpInput.to || ''}
                  onChange={this.handleChangeTmpInput.bind(this, 'to')}
                  onPressEnter={this.handleAddField.bind(this, 'content_replace')}
                  suffix={
                    <a onClick={this.handleAddField.bind(this, 'content_replace')}>按回车添加</a>
                  }
                />
              </Input.Group>
            </ProFormText>
          </ModalForm>
        )}
      </>
    );
  }
}

export default CollectorSetting;
