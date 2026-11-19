/**
 * 仪表盘「新站点操作指南」。
 *
 * 普通版与商城版是两份不同的引导内容（合并前分别位于前端独立仓库的
 * main / mall 分支）：商城版含「添加第一个产品 / 支付 / 物流」等入口，
 * 这些路由在普通内容模式下整组隐藏，点了就是不可达页面。
 * 因此按 shopMode 分流，各渲染各的，不再共用一份商城版引导。
 *
 * 两份内容都走 contentEditPath() 取发布入口，与项目里其余模式分流保持一致。
 */
import { brand } from '@/oem/brand';
import { contentEditPath } from '@/utils/contentRoutes';
import {
  FileAddOutlined,
  LayoutOutlined,
  LinkOutlined,
  MoneyCollectOutlined,
  ProductOutlined,
  SendOutlined,
  ShopOutlined,
  TagOutlined,
} from '@ant-design/icons';
import { Button, Card, Col, Row, Space } from 'antd';
import { FormattedMessage } from '@umijs/max';
import React from 'react';

type GuideProps = {
  shopMode: boolean;
  onJump: (path: string) => void;
  onPreview: () => void;
  onClose: () => void;
};

/** 普通内容模式：来自合并前 main 分支的原始引导，链接换成当前有效路由 */
const NormalGuide: React.FC<Omit<GuideProps, 'shopMode'>> = ({
  onJump,
  onPreview,
  onClose,
}) => (
  <Card className="guide-card" style={{ marginBottom: 24 }}>
    <div className="guide-header">
      <h1>
        <FormattedMessage id="dashboard.guide.welcome" />
      </h1>
      <p>
        <FormattedMessage id="dashboard.guide.description" />
      </p>
      <span className="guild-close" onClick={onClose}>
        <FormattedMessage id="component.close" />
      </span>
    </div>
    <Row gutter={16}>
      <Col flex={1}>
        <div className="guide-start">
          <h2>
            <FormattedMessage id="dashboard.guide.welcome" />
          </h2>
          <div className="main-start">
            <Button
              size="large"
              type="primary"
              onClick={() => onJump('/setting/system')}
            >
              <FormattedMessage id="dashboard.guide.system" />
            </Button>
          </div>
          <div>
            <FormattedMessage id="dashboard.guide.or" />
            <span className="link" onClick={() => onJump('/design/index')}>
              <FormattedMessage id="dashboard.guide.design" />
            </span>
            <FormattedMessage id="dashboard.guide.comma" />
            <a
              className="link"
              href={`${brand.docs}/help`}
              target="_blank"
              rel="noreferrer"
            >
              <FormattedMessage id="dashboard.guide.help" />
            </a>
          </div>
        </div>
      </Col>
      <Col flex={1}>
        <div className="guide-start">
          <h2>
            <FormattedMessage id="dashboard.guide.next" />
          </h2>
          <ul className="guide-list">
            <li
              className="link"
              onClick={() => onJump('/archive/category')}
            >
              <FormattedMessage id="dashboard.guide.new-category" />
            </li>
            <li className="link" onClick={() => onJump(contentEditPath())}>
              <FormattedMessage id="dashboard.guide.new-archive" />
            </li>
            <li className="link" onClick={() => onJump('/archive/page')}>
              <FormattedMessage id="dashboard.guide.new-page" />
            </li>
            <li className="link" onClick={() => onJump('/setting/tdk')}>
              <FormattedMessage id="dashboard.guide.tdk" />
            </li>
            <li className="link" onClick={onPreview}>
              <FormattedMessage id="dashboard.guide.preview" />
            </li>
          </ul>
        </div>
      </Col>
      <Col flex={1}>
        <div className="guide-start">
          <h2>
            <FormattedMessage id="dashboard.guide.step" />
          </h2>
          <Row>
            <Col flex={1}>
              <ul className="guide-list split">
                <li>
                  1.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/system')}
                  >
                    <FormattedMessage id="menu.setting.system" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.website" />
                </li>
                <li>
                  2.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/content')}
                  >
                    <FormattedMessage id="menu.setting.content" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.website" />
                </li>
                <li>
                  3.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/tdk')}
                  >
                    <FormattedMessage id="menu.setting.tdk" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  4.{' '}
                  <FormattedMessage id="dashboard.guide.step.create" />
                  <span
                    className="link"
                    onClick={() => onJump('/archive/category')}
                  >
                    <FormattedMessage id="menu.archive.category" />
                  </span>
                  ；{' '}
                </li>
                <li>
                  5.{' '}
                  <span className="link" onClick={() => onJump('/archive/page')}>
                    <FormattedMessage id="dashboard.guide.step.create" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.page" />
                </li>
              </ul>
            </Col>
            <Col flex={1}>
              <ul className="guide-list split">
                <li>
                  6.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/nav')}
                  >
                    <FormattedMessage id="menu.setting.nav" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  7.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/plugin/seo/rewrite')}
                  >
                    <FormattedMessage id="menu.plugin.seo.rewrite" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  8.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/plugin/seo/robots')}
                  >
                    <FormattedMessage id="menu.plugin.seo.robots" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  9.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/plugin/seo/sitemap')}
                  >
                    <FormattedMessage id="menu.plugin.seo.sitemap" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  10.{' '}
                  <span className="link" onClick={() => onJump(contentEditPath())}>
                    <FormattedMessage id="dashboard.guide.step.publish" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.dot" />
                </li>
              </ul>
            </Col>
          </Row>
        </div>
      </Col>
    </Row>
  </Card>
);

/** 商城模式：来自合并前 mall 分支的原始引导（含产品/支付/物流入口） */
const ShopGuide: React.FC<Omit<GuideProps, 'shopMode'>> = ({
  onJump,
  onPreview,
  onClose,
}) => (
  <Card className="guide-card" style={{ marginBottom: 24 }}>
    <div className="guide-header">
      <h1>
        <FormattedMessage id="dashboard.guide.name" />
      </h1>
      <p>
        <FormattedMessage id="dashboard.guide.description" />
        <span>
          <a
            className="link"
            href={`${brand.docs}/help`}
            target="_blank"
            rel="noreferrer"
          >
            <FormattedMessage id="dashboard.guide.help" />
          </a>
        </span>
      </p>
      <span className="guild-close" onClick={onClose}>
        <FormattedMessage id="component.close" />
      </span>
    </div>
    <Row gutter={16}>
      <Col flex={1}>
        <div className="guide-start">
          <h2>
            <ShopOutlined />{' '}
            <FormattedMessage id="dashboard.guide.welcome" />
          </h2>
          <div className="main-start">
            <Button
              size="large"
              type="primary"
              onClick={() => onJump('/setting/system')}
            >
              <FormattedMessage id="dashboard.guide.system" />
            </Button>
          </div>
        </div>
      </Col>
      <Col flex={1}>
        <Space
          className="add-design"
          direction="vertical"
          size={16}
          style={{ width: '100%' }}
        >
          <div>
            <h3>
              <ProductOutlined />{' '}
              <FormattedMessage id="dashboard.guide.product" />
            </h3>
            <div className="text-muted">
              <FormattedMessage id="dashboard.guide.product.tips" />
            </div>
          </div>
          <div className="show-button">
            <Button size="large" type="primary">
              <FormattedMessage id="dashboard.guide.product.start" />
            </Button>
          </div>
        </Space>
      </Col>
      <Col flex={1}>
        <Space
          className="add-design"
          direction="vertical"
          size={16}
          style={{ width: '100%' }}
        >
          <div>
            <h3>
              <LayoutOutlined />{' '}
              <FormattedMessage id="dashboard.guide.design" />
            </h3>
            <div className="text-muted">
              <FormattedMessage id="dashboard.guide.design.tips" />
            </div>
          </div>
          <div className="show-button">
            <Button size="large" type="primary">
              <FormattedMessage id="dashboard.guide.design.start" />
            </Button>
          </div>
        </Space>
      </Col>
    </Row>
    <Row gutter={16} className="mt-normal">
      <Col flex={1}>
        <div className="guide-start">
          <h2>
            <FormattedMessage id="dashboard.guide.next" />
          </h2>
          <ul className="guide-list">
            <li className="link" onClick={() => onJump('/setting/pay')}>
              <MoneyCollectOutlined />{' '}
              <FormattedMessage id="dashboard.guide.set-payment" />
            </li>
            <li className="link" onClick={() => onJump('/order/shipping')}>
              <SendOutlined />{' '}
              <FormattedMessage id="dashboard.guide.shipping" />
            </li>
            <li className="link" onClick={() => onJump('/archive/page')}>
              <FileAddOutlined />{' '}
              <FormattedMessage id="dashboard.guide.new-page" />
            </li>
            <li className="link" onClick={() => onJump('/setting/tdk')}>
              <TagOutlined />{' '}
              <FormattedMessage id="dashboard.guide.tdk" />
            </li>
            <li className="link" onClick={onPreview}>
              <LinkOutlined />{' '}
              <FormattedMessage id="dashboard.guide.preview" />
            </li>
          </ul>
        </div>
      </Col>
      <Col flex={1}>
        <div className="guide-start">
          <h2>
            <FormattedMessage id="dashboard.guide.step" />
          </h2>
          <Row>
            <Col flex={1}>
              <ul className="guide-list split">
                <li>
                  1.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/system')}
                  >
                    <FormattedMessage id="menu.setting.system" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.website" />
                </li>
                <li>
                  2.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/content')}
                  >
                    <FormattedMessage id="menu.setting.content" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.website" />
                </li>
                <li>
                  3.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/tdk')}
                  >
                    <FormattedMessage id="menu.setting.tdk" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  4.{' '}
                  <FormattedMessage id="dashboard.guide.step.create" />
                  <span
                    className="link"
                    onClick={() => onJump('/archive/category')}
                  >
                    <FormattedMessage id="dashboard.guide.category" />
                  </span>
                  ；{' '}
                </li>
                <li>
                  5.{' '}
                  <span className="link" onClick={() => onJump('/archive/page')}>
                    <FormattedMessage id="dashboard.guide.step.create" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.page" />
                </li>
              </ul>
            </Col>
            <Col flex={1}>
              <ul className="guide-list split">
                <li>
                  6.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/setting/nav')}
                  >
                    <FormattedMessage id="menu.setting.nav" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  7.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/plugin/seo/rewrite')}
                  >
                    <FormattedMessage id="menu.plugin.seo.rewrite" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  8.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/plugin/seo/robots')}
                  >
                    <FormattedMessage id="menu.plugin.seo.robots" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  9.{' '}
                  <FormattedMessage id="dashboard.guide.step.change" />
                  <span
                    className="link"
                    onClick={() => onJump('/plugin/seo/sitemap')}
                  >
                    <FormattedMessage id="menu.plugin.seo.sitemap" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.semicolon" />
                </li>
                <li>
                  10.{' '}
                  <span className="link" onClick={() => onJump(contentEditPath())}>
                    <FormattedMessage id="dashboard.guide.step.publish" />
                  </span>
                  <FormattedMessage id="dashboard.guide.step.dot" />
                </li>
              </ul>
            </Col>
          </Row>
        </div>
      </Col>
    </Row>
  </Card>
);

const Guide: React.FC<GuideProps> = ({ shopMode, ...rest }) =>
  shopMode ? <ShopGuide {...rest} /> : <NormalGuide {...rest} />;

export default Guide;
