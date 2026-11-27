import { anqiRecharge, anqiRechargeStatus, getAnqiPackages } from '@/services';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import {
  Button,
  Empty,
  InputNumber,
  Radio,
  Result,
  Space,
  Spin,
  message,
} from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

/** 充值类型：vip=续期用户组，integral=充积分（与官网 /auth/recharge 的 type 一致） */
export type VipPurchaseType = 'vip' | 'integral';

interface VipPurchaseProps {
  /** 默认买 VIP；积分充值传 integral */
  type?: VipPurchaseType;
  /** 关闭弹窗，由父组件决定怎么关 */
  onClose?: () => void;
  /** 支付成功且本地账号信息已刷新后触发 */
  onSuccess?: () => void;
}

/**
 * 后台内嵌的购买面板，VIP 续期与积分充值共用。
 *
 * 卖点在于「不跳官网」：OEM 品牌下 brand.website 是客户自己的站点，那里没有
 * 账号/充值页面，任何 window.open 都是死路。资金流仍是官网代收，这里只负责
 * 展示套餐、下单、渲染微信二维码与轮询结果。
 *
 * 由父组件在弹窗打开时挂载（`{open && <VipPurchase />}`），挂载即重新拉套餐，
 * 关闭卸载即清空状态，不需要额外的重置逻辑。
 */
const VipPurchase: React.FC<VipPurchaseProps> = ({
  type = 'vip',
  onClose,
  onSuccess,
}) => {
  const intl = useIntl();
  const { initialState, setInitialState } = useModel('@@initialState');
  const [packages, setPackages] = useState<any>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [goodsId, setGoodsId] = useState<number>(0);
  const [quantity, setQuantity] = useState<number>(1);
  const [payWay, setPayWay] = useState<string>('wechat');
  const [order, setOrder] = useState<any>(null);
  const [paying, setPaying] = useState<boolean>(false);
  const [paid, setPaid] = useState<boolean>(false);

  const isIntegral = type === 'integral';
  const vipPackages: any[] = packages?.vip || [];
  // 积分是官网的单一商品，goods_id 恒为 0，靠数量决定金额。
  const integralPackage = packages?.integral;
  // VIP 一年一档，数量恒为 1；积分按份买，数量由用户填。
  const buyQuantity = isIntegral ? quantity : 1;
  const unitPrice: number = isIntegral
    ? integralPackage?.price || 0
    : vipPackages.find((p) => p.goods_id === goodsId)?.price || 0;
  const available = isIntegral
    ? Boolean(integralPackage) && unitPrice > 0
    : vipPackages.length > 0 && Boolean(goodsId);
  // 官网目前只实现了微信扫码，单渠道时不显示支付方式选择。
  const payWays = packages?.pay_ways?.length ? packages.pay_ways : ['wechat'];

  // 套餐价格随官网配置变化，每次挂载都重新拉，不做本地缓存。
  useEffect(() => {
    let alive = true;
    const load = async () => {
      setLoading(true);
      try {
        const res = await getAnqiPackages({});
        if (!alive) {
          return;
        }
        if (res.code !== 0) {
          message.error(res.msg);
          return;
        }
        setPackages(res.data || null);
        const first = res.data?.vip?.[0];
        if (first) {
          setGoodsId(first.goods_id);
        }
        const ways = res.data?.pay_ways;
        if (ways?.length) {
          setPayWay(ways[0]);
        }
      } catch (e) {
        // 网络异常时保持空态，用户可关掉重开
      } finally {
        if (alive) {
          setLoading(false);
        }
      }
    };
    load();
    return () => {
      alive = false;
    };
  }, []);

  // 支付结果只能轮询：OEM 客户多为内网部署，官网的回调打不进去。
  // 节奏按官网契约建议：3 秒一次，最多 40 次（约 2 分钟）。
  useEffect(() => {
    if (!order?.order_id || paid) {
      return;
    }
    let tries = 0;
    const timer = setInterval(async () => {
      tries += 1;
      if (tries > 40) {
        clearInterval(timer);
        message.warning(
          intl.formatMessage({ id: 'component.right-content.vip.pay.timeout' }),
        );
        return;
      }
      try {
        const res = await anqiRechargeStatus({ order_id: order.order_id });
        if (res.code === 0 && res.data?.paid) {
          clearInterval(timer);
          setPaid(true);
          message.success(
            intl.formatMessage({
              id: isIntegral
                ? 'component.right-content.integral.pay.success'
                : 'component.right-content.vip.pay.success',
            }),
          );
          const user = await initialState?.fetchAnqiUser?.();
          if (user) {
            await setInitialState((s) => ({
              ...s,
              anqiUser: user,
            }));
          }
          onSuccess?.();
        }
      } catch (e) {
        // 网络抖动不打断轮询，下一轮再试
      }
    }, 3000);
    return () => clearInterval(timer);
  }, [order?.order_id, paid]);

  const handlePay = async () => {
    if (!available) {
      message.error(
        intl.formatMessage({ id: 'component.right-content.vip.empty' }),
      );
      return;
    }
    setPaying(true);
    try {
      const res = await anqiRecharge({
        type,
        // 积分商品的 goods_id 恒为 0，官网按 IntegralGoods 结算，不看这个字段
        goods_id: isIntegral ? 0 : goodsId,
        quantity: buyQuantity,
        pay_way: payWay,
      });
      if (res.code !== 0) {
        message.error(res.msg);
        return;
      }
      setOrder(res.data);
      // 支付宝走收银台跳转，微信走扫码（二维码直接在弹窗里显示）
      if (res.data?.jump_url) {
        window.open(res.data.jump_url);
      }
    } finally {
      setPaying(false);
    }
  };

  const payWayLabel = (way: string) =>
    way === 'alipay'
      ? intl.formatMessage({ id: 'component.right-content.vip.pay.alipay' })
      : intl.formatMessage({ id: 'component.right-content.vip.pay.wechat' });

  if (paid) {
    return (
      <Result
        status="success"
        title={intl.formatMessage({
          id: isIntegral
            ? 'component.right-content.integral.pay.success'
            : 'component.right-content.vip.pay.success',
        })}
        extra={
          <Button type="primary" onClick={onClose}>
            <FormattedMessage id="component.right-content.vip.pay.done" />
          </Button>
        }
      />
    );
  }

  if (order?.code_url) {
    return (
      <div className="vip-purchase-center">
        <img src={order.code_url} alt="pay" className="vip-purchase-qrcode" />
        <p>
          <FormattedMessage id="component.right-content.vip.pay.scan" />
        </p>
        <p className="vip-purchase-muted">
          <FormattedMessage id="component.right-content.vip.pay.amount" />
          {` ¥${((order.amount_fen ?? 0) / 100).toFixed(2)}`}
        </p>
        <Space align="center">
          <Spin size="small" />
          <FormattedMessage id="component.right-content.vip.pay.waiting" />
        </Space>
      </div>
    );
  }

  const payWayPicker = payWays.length > 1 && (
    <>
      <div className="vip-purchase-title">
        <FormattedMessage id="component.right-content.vip.pay.way" />
      </div>
      <Radio.Group
        value={payWay}
        onChange={(e) => {
          setPayWay(e.target.value);
        }}
      >
        {payWays.map((way: string) => (
          <Radio key={way} value={way}>
            {payWayLabel(way)}
          </Radio>
        ))}
      </Radio.Group>
    </>
  );

  const actions = (
    <div className="vip-purchase-actions">
      <Space align="center">
        <Button onClick={onClose}>
          <FormattedMessage id="component.close" />
        </Button>
        <Button type="primary" loading={paying} onClick={handlePay}>
          <FormattedMessage id="component.right-content.vip.pay.now" />
        </Button>
      </Space>
    </div>
  );

  return (
    <Spin spinning={loading}>
      {!available ? (
        <Empty
          description={intl.formatMessage({
            id: 'component.right-content.vip.empty',
          })}
        />
      ) : isIntegral ? (
        <>
          <div className="vip-purchase-title">
            <FormattedMessage id="component.right-content.integral.package" />
          </div>
          <div className="vip-purchase-muted">
            <FormattedMessage id="component.right-content.integral.price" />
            {` ¥${(unitPrice / 100).toFixed(2)}`}
          </div>
          <div className="vip-purchase-muted">
            <FormattedMessage id="component.right-content.integral.arrive" />
            {` ${integralPackage.total} `}
            <FormattedMessage id="component.right-content.integral.points" />
          </div>
          <div className="vip-purchase-title">
            <FormattedMessage id="component.right-content.integral.quantity" />
          </div>
          <InputNumber
            min={1}
            max={999}
            precision={0}
            value={quantity}
            onChange={(v) => {
              setQuantity(Number(v) || 1);
            }}
          />
          <div className="vip-purchase-title">
            <FormattedMessage id="component.right-content.integral.total" />
          </div>
          <div>
            {`¥${((unitPrice * buyQuantity) / 100).toFixed(2)}`}
            <span className="vip-purchase-muted">
              {` / ${integralPackage.total * buyQuantity} `}
              <FormattedMessage id="component.right-content.integral.points" />
            </span>
          </div>
          {payWayPicker}
          {actions}
        </>
      ) : (
        <>
          <div className="vip-purchase-title">
            <FormattedMessage id="component.right-content.vip.package" />
          </div>
          <Radio.Group
            value={goodsId}
            onChange={(e) => {
              setGoodsId(e.target.value);
            }}
            style={{ display: 'block' }}
          >
            <Space direction="vertical">
              {vipPackages.map((p: any) => (
                <Radio key={p.goods_id} value={p.goods_id}>
                  {p.title}
                  <span className="vip-purchase-muted">
                    {` ¥${(p.price / 100).toFixed(2)} / ${p.unit}`}
                  </span>
                </Radio>
              ))}
            </Space>
          </Radio.Group>
          {payWayPicker}
          {actions}
        </>
      )}
    </Spin>
  );
};

export default VipPurchase;
