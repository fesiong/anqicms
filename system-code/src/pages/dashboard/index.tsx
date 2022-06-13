import React, { Suspense, useEffect, useState } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import { Card, Row, Col, Statistic, Tabs } from 'antd';
import './index.less';
import StatisticsRow from './components/statistics';
import { getDashboardInfo, getStatisticInclude, getStatisticSpider, getStatisticSummary, getStatisticTraffic } from '@/services';
import { history } from 'umi';
import { Line } from '@ant-design/plots';
import moment from 'moment';

const { TabPane } = Tabs;

const Dashboard: React.FC = () => {
  const [data, setData] = useState<any>({});
  const [includeData, setIncludeData] = useState<any[]>([]);
  const [spiderData, setSpiderData] = useState<any[]>([]);
  const [trafficData, setTrafficData] = useState<any[]>([]);
  const [infoData, setInfoData] = useState<any>({});


  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    getStatisticSummary().then(res => {
      setData(res.data || {})
    });
    getStatisticInclude().then(res => {
      setIncludeData(res.data || [])
    })
    getStatisticTraffic().then(res => {
      setTrafficData(res.data || [])
    })
    getStatisticSpider().then(res => {
      setSpiderData(res.data || [])
    })
    getDashboardInfo().then(res => {
      setInfoData(res.data || [])
    })
  };

  const handleJump = (str: string) => {
    history.push(str);
  }

  const includeConfig = {
    data: includeData,
    //padding: 'auto',
    xField: 'date',
    yField: 'value',
    seriesField: 'label',
    xAxis: {
      // type: 'timeCat',
      tickCount: 5,
    },
    smooth: true,
  };

  const trafficConfig = {
    data: trafficData,
    //padding: 'auto',
    xField: 'date',
    yField: 'value',
    seriesField: 'label',
    xAxis: {
      // type: 'timeCat',
      tickCount: 5,
    },
    smooth: true,
  };

  const spiderConfig = {
    data: spiderData,
    //padding: 'auto',
    xField: 'date',
    yField: 'value',
    seriesField: 'label',
    xAxis: {
      // type: 'timeCat',
      tickCount: 5,
    },
    smooth: true,
  };

  return (
    <PageContainer>
       <Suspense fallback={null}>
         <StatisticsRow loading={false} data={data}  />
         </Suspense>
         <Row gutter={20}>
           <Col span={18}>
           <Card title='快捷操作'>
          <Row gutter={16}>
          {data.archive_counts?.map((item: any, index: number) => (
            <Col flex={1} key={index} onClick={() => {handleJump('/archive/list')}}>
            <Statistic className='link' title={item.name} value={item.total} />
          </Col>
        ))}
          <Col flex={1} onClick={() => {handleJump('/archive/category')}}>
            <Statistic className='link' title='文档分类' value={data.category_count} />
          </Col>
          <Col flex={1} onClick={() => {handleJump('/plugin/friendlink')}}>
            <Statistic className='link' title='友情链接' value={data.link_count} />
          </Col>
          <Col flex={1} onClick={() => {handleJump('/plugin/guestbook')}}>
            <Statistic className='link' title='网站留言' value={data.guestbook_count} />
          </Col>
          <Col flex={1} onClick={() => {handleJump('/content/page')}}>
            <Statistic className='link' title='单页管理' value={data.page_count} />
          </Col>
          <Col flex={1} onClick={() => {handleJump('/content/attachment')}}>
            <Statistic className='link' title='图片管理' value={data.attachment_count} />
          </Col>
          <Col flex={1} onClick={() => {handleJump('/design/index')}}>
            <Statistic className='link' title='模板设计' value={data.template_count} />
          </Col>
          </Row>
         </Card>
         <Suspense fallback={null}>
         <Card style={{marginTop: '24px'}} bordered={false} bodyStyle={{ padding: '0 24px 24px' }}>
    <div className='statistic-card'>
      <Tabs
        size="large"
        tabBarStyle={{ marginBottom: 24 }}
      >
        <TabPane tab="访问量" key="traffic">
        <div className='statistic-bar'>
          <Line {...trafficConfig} />
              </div>
        </TabPane>
        <TabPane tab="蜘蛛爬行" key="spider">
        <div className='statistic-bar'>
        <Line {...spiderConfig} />
              </div>
        </TabPane>
        <TabPane tab="网站收录" key="include">
        <div className='statistic-bar'>
        <Line {...includeConfig} />
              </div>
        </TabPane>
      </Tabs>
    </div>
  </Card>
        </Suspense>
           </Col>
           <Col span={6}>
             <Card title='登录信息'>
               <Row gutter={24}>
                 <Col span={12}>
                   <div className='info-card'>
                     <div className='title'>本次登录</div>
                     <p>时间：{infoData.now_login ? moment(infoData.now_login.created_time * 1000).format('MM-DD HH:mm') : '-'}</p>
                    <div>IP：{infoData.now_login?.ip}</div>
                   </div>
                 </Col>
                 <Col span={12}>
                  <div className='info-card'>
                     <div className='title'>上次登录</div>
                     <p>时间：{infoData.last_login ? moment(infoData.last_login.created_time * 1000).format('MM-DD HH:mm') : '-'}</p>
                    <div>IP：{infoData.last_login?.ip}</div>
                   </div>
                 </Col>
               </Row>
             </Card>
             <Card style={{marginTop: '24px'}} title='网站信息'>
             <Row gutter={[16,24]}>
                 <Col span={12}>
                 <div className='info-card'>
                     <div className='title'>网站名称</div>
                     <div>{infoData.system?.site_name}</div>
                   </div>
                 </Col>
                 <Col span={12}>
                 <div className='info-card'>
                     <div className='title'>网站地址</div>
                     <div>{infoData.system?.base_url}</div>
                   </div>
                 </Col>
                 <Col span={12}>
                  <div className='info-card'>
                     <div className='title'>移动端地址</div>
                     <div>{infoData.system?.mobile_url || '未设置'}</div>
                   </div>
                 </Col>
                 <Col span={12}>
                 <div className='info-card'>
                     <div className='title'>网站类型</div>
                     <div>{infoData.system?.template_type == 2 ? '电脑+手机' : infoData.system?.template_type == 1 ? '代码适配' : '自适应'}</div>
                   </div>
                 </Col>
               </Row>
             </Card>
             <Card style={{marginTop: '24px'}} title='软件信息'>
               <p>软件版本：{infoData.version}</p>
               <p>官网地址：<a href='https://www.kandaoni.com/' target={'_blank'}>https://www.kandaoni.com</a></p>
               <div>安企内容管理系统(AnqiCMS)，是一款使用 GoLang 开发的企业站内容管理系统，它部署简单，软件安全，界面优雅，小巧，执行速度飞快，使用 AnqiCMS 搭建的网站可以防止众多安全问题发生。</div>
             </Card>
           </Col>
         </Row>
    </PageContainer>
  );
};

export default Dashboard;
