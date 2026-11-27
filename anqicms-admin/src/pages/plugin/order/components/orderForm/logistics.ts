export type OrderLogisticsField = {
  labelKey: string;
  value: string;
};

// 订单详情要展示数据库里已经保存的快递公司/运单号。
// 抽成纯函数便于用 node:test 锁住“缺字段回退 -”的行为。
export function orderLogisticsFields(order: any): OrderLogisticsField[] {
  return [
    {
      labelKey: 'plugin.order.express-company',
      value: order?.express_company || '-',
    },
    {
      labelKey: 'plugin.order.tracking-number',
      value: order?.tracking_number || '-',
    },
  ];
}
