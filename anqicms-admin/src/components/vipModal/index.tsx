import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { Modal } from 'antd';
import React, { useCallback, useRef, useState } from 'react';
import VipPurchase from '../vipPurchase';

interface VipModalResult {
  /** 当前用户是否为有效 VIP */
  isVip: boolean;
  /**
   * 检查 VIP 权限，如果不是 VIP 则弹出购买弹窗
   * @param action VIP 验证通过后执行的回调
   * @param text 显示文本
   * @returns true=是VIP已执行回调, false=不是VIP已弹窗
   */
  checkVip: (action?: () => void, text?: React.ReactNode | string) => boolean;
  /** 在调用方 JSX 中渲染的 VIP 弹窗组件 */
  VipModal: React.FC;
}

export function useVipModal(): VipModalResult {
  const { initialState } = useModel('@@initialState');
  const [visible, setVisible] = useState(false);
  const [text, setText] = useState<React.ReactNode | string>(null);
  const actionRef = useRef<(() => void) | undefined>();
  const intl = useIntl();

  const isVip = initialState?.anqiUser?.valid === true;

  const checkVip = useCallback(
    (action?: () => void, text?: React.ReactNode | string) => {
      if (isVip) {
        action?.();
        return true;
      }
      actionRef.current = action;
      setText(text);
      setVisible(true);
      return false;
    },
    [isVip],
  );

  const handleCancel = () => {
    setVisible(false);
    actionRef.current = undefined;
  };

  // 购买直接在后台完成：OEM 品牌下 brand.website 是客户自己的站点，
  // 那里没有 /account/vip 这个页面，任何跳转都是死路。
  const VipModal: React.FC = () => (
    <Modal
      title={intl.formatMessage({ id: 'component.right-content.vip-title' })}
      open={visible}
      onCancel={handleCancel}
      width={500}
      footer={null}
    >
      <div className="vip-modal-content">
        {text && <div className="mb-normal">{text}</div>}
        <p>
          <FormattedMessage id="component.right-content.vip-tips" />
        </p>
        {visible && <VipPurchase onClose={handleCancel} />}
      </div>
    </Modal>
  );

  return { isVip, checkVip, VipModal };
}
