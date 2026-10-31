import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { Button, Modal } from 'antd';
import React, { useCallback, useRef, useState } from 'react';

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

  const handleOrderVip = () => {
    window.open('https://www.anqicms.com/account/vip');
  };

  const handleCancel = () => {
    setVisible(false);
    actionRef.current = undefined;
  };

  const VipModal: React.FC = () => (
    <Modal
      title={intl.formatMessage({ id: 'component.right-content.vip-title' })}
      open={visible}
      onCancel={handleCancel}
      width={500}
      footer={
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <Button onClick={handleCancel}>
            <FormattedMessage id="component.close" />
          </Button>
          <Button type="primary" onClick={handleOrderVip}>
            <FormattedMessage id="component.right-content.order" />
          </Button>
        </div>
      }
    >
      <div className="vip-modal-content">
        {text && <div className="mb-normal">{text}</div>}
        <p>
          <FormattedMessage id="component.right-content.vip-tips" />
        </p>
        <p>
          <FormattedMessage id="component.right-content.order.confirm" />
          <Button type="link" onClick={handleOrderVip}>
            <FormattedMessage id="component.right-content.order" />
          </Button>
        </p>
        <p>
          <FormattedMessage id="component.right-content.vip-prefix" />
          <a
            href="https://www.anqicms.com/account/vip"
            target="_blank"
            rel="noreferrer"
          >
            <FormattedMessage id="component.right-content.vip-name" />
          </a>
          <FormattedMessage id="component.right-content.vip-suffix" />
        </p>
      </div>
    </Modal>
  );

  return { isVip, checkVip, VipModal };
}
