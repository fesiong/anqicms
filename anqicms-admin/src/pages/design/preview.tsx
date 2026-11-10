import AiChat from '@/components/aiChat';
import { getDesignInfo } from '@/services';
import {
  AimOutlined,
  ArrowLeftOutlined,
  DesktopOutlined,
  GlobalOutlined,
  MobileOutlined,
  ReloadOutlined,
  RobotOutlined,
} from '@ant-design/icons';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { Button, Space, Tag, Tooltip, message } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import './preview.less';

// agent.js 注入到 iframe 内的脚本，负责元素选择器、URL 上报、<a> 拦截、postMessage origin 校验
// 元素选择器采用 DevTools 风格：父页面发送 enable-pick 指令激活，
// 激活后鼠标悬停高亮元素（半透明蓝色覆盖层 + 元素标签），
// 点击选中元素并发送 pick-element 消息，按 Esc 取消。
// 选中后被选元素显示持久的橙色边框高亮。
const AGENT_JS_CODE = `(function(){
  if(window.__anqi_agent_injected__) return;
  window.__anqi_agent_injected__ = true;
  var PARENT_ORIGIN = location.origin;
  var pickMode = false;       // 是否处于元素选择模式
  var hoveredEl = null;       // 当前悬停的元素
  var selectedEl = null;      // 当前选中的元素
  var overlay = null;         // 悬停高亮覆盖层
  var label = null;           // 悬停元素标签
  var selectedBox = null;     // 选中元素的持久高亮边框

  function send(type, data){
    try{ window.parent.postMessage(Object.assign({type:type}, data||{}), PARENT_ORIGIN); }catch(e){}
  }

  // 创建覆盖层和标签（惰性初始化）
  function ensureOverlay(){
    if(overlay) return;
    overlay = document.createElement('div');
    overlay.style.cssText = 'position:fixed;z-index:2147483646;pointer-events:none;border:1px solid #1a73e8;background:rgba(26,115,232,0.15);transition:all 0.05s ease;';
    document.body.appendChild(overlay);
    label = document.createElement('div');
    label.style.cssText = 'position:fixed;z-index:2147483647;pointer-events:none;background:#1a73e8;color:#fff;font:11px/1.4 monospace;padding:2px 6px;border-radius:3px 3px 3px 0;max-width:80vw;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;';
    document.body.appendChild(label);
  }

  function removeOverlay(){
    if(overlay){ overlay.style.display='none'; }
    if(label){ label.style.display='none'; }
  }

  // 生成元素的 CSS 选择器描述（用于 AI 定位）
  function describeEl(el){
    var parts = [];
    var cur = el;
    while(cur && cur !== document.body && cur !== document.documentElement){
      var part = cur.nodeName.toLowerCase();
      if(cur.id){ part += '#' + cur.id; }
      else if(cur.className && typeof cur.className === 'string'){
        var cls = cur.className.trim().split(/\\s+/).filter(Boolean).slice(0,2).join('.');
        if(cls) part += '.' + cls;
      }
      parts.unshift(part);
      cur = cur.parentNode;
    }
    return parts.join(' > ');
  }

  // 获取元素的 outerHTML（限制长度避免消息过大）
  function getOuterHtml(el){
    try{
      var clone = el.cloneNode(true);
      var div = document.createElement('div');
      div.appendChild(clone);
      var html = div.innerHTML;
      // 截断过长的 HTML
      if(html.length > 2000) html = html.substring(0, 2000) + '...';
      return html;
    }catch(e){ return ''; }
  }

  // 进入元素选择模式
  function enablePick(){
    pickMode = true;
    ensureOverlay();
    // 清除上一次的选中高亮
    if(selectedBox){ selectedBox.parentNode.removeChild(selectedBox); selectedBox=null; }
    // 阻止文字选取
    document.body.style.userSelect = 'none';
    document.body.style.webkitUserSelect = 'none';
    document.body.style.cursor = 'crosshair';
  }

  // 退出元素选择模式
  function disablePick(){
    pickMode = false;
    hoveredEl = null;
    removeOverlay();
    document.body.style.userSelect = '';
    document.body.style.webkitUserSelect = '';
    document.body.style.cursor = '';
  }

  // 更新持久选中边框位置（跟随目标元素滚动）
  function updateSelectedBox(){
    if(!selectedBox || !selectedEl || !selectedEl.parentNode) return;
    var rect = selectedEl.getBoundingClientRect();
    if(rect.width === 0 && rect.height === 0){
      selectedBox.style.display = 'none';
      return;
    }
    selectedBox.style.display = 'block';
    selectedBox.style.left = rect.left + 'px';
    selectedBox.style.top = rect.top + 'px';
    selectedBox.style.width = rect.width + 'px';
    selectedBox.style.height = rect.height + 'px';
  }

  // scroll/resize 时更新选中边框位置（捕获阶段，尽早响应）
  document.addEventListener('scroll', function(){
    if(selectedBox) updateSelectedBox();
    if(overlay && hoveredEl){
      var rect = hoveredEl.getBoundingClientRect();
      overlay.style.left = rect.left + 'px';
      overlay.style.top = rect.top + 'px';
      overlay.style.width = rect.width + 'px';
      overlay.style.height = rect.height + 'px';
      label.style.left = rect.left + 'px';
      label.style.top = Math.max(0, rect.top - 20) + 'px';
    }
  }, true);
  window.addEventListener('resize', function(){
    if(selectedBox) updateSelectedBox();
  });

  // 元素选择模式：鼠标移动高亮元素
  document.addEventListener('mousemove', function(e){
    if(!pickMode) return;
    var el = e.target;
    if(!el || el === overlay || el === label || el === selectedBox) return;
    if(el === hoveredEl) return;
    hoveredEl = el;
    var rect = el.getBoundingClientRect();
    overlay.style.display = 'block';
    overlay.style.left = rect.left + 'px';
    overlay.style.top = rect.top + 'px';
    overlay.style.width = rect.width + 'px';
    overlay.style.height = rect.height + 'px';
    label.style.display = 'block';
    label.style.left = rect.left + 'px';
    label.style.top = Math.max(0, rect.top - 20) + 'px';
    label.textContent = describeEl(el);
  }, true);

  // 统一的 click 拦截器（捕获阶段）
  // 处理：元素选择模式下的点击选中 + <a> 标签跳转拦截
  document.addEventListener('click', function(e){
    // 元素选择模式：点击选中元素，阻止一切默认行为
    if(pickMode){
      e.preventDefault();
      e.stopPropagation();
      var el = e.target;
      if(!el || el === overlay || el === label) return;
      selectedEl = el;
      var rect = el.getBoundingClientRect();
      send('pick-element', {
        html: getOuterHtml(el),
        xpath: describeEl(el),
        tagName: el.nodeName.toLowerCase(),
        rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height, top: rect.top, left: rect.left }
      });
      // 显示持久选中边框
      if(selectedBox){ selectedBox.parentNode.removeChild(selectedBox); }
      selectedBox = document.createElement('div');
      selectedBox.style.cssText = 'position:fixed;z-index:2147483645;pointer-events:none;border:2px solid #ff6b00;background:rgba(255,107,0,0.1);transition:all 0.1s ease;';
      selectedBox.style.display = 'block';
      selectedBox.style.left = rect.left + 'px';
      selectedBox.style.top = rect.top + 'px';
      selectedBox.style.width = rect.width + 'px';
      selectedBox.style.height = rect.height + 'px';
      document.body.appendChild(selectedBox);
      // 选中后退出选择模式
      disablePick();
      return;
    }
    // 非选择模式：<a> 标签跳转拦截
    var a = e.target;
    while(a && a.tagName !== 'A'){ a = a.parentNode; }
    if(!a) return;
    var href = a.getAttribute('href');
    if(!href || href.charAt(0) === '#') return;
    // download 链接放行（触发浏览器下载）
    if(a.hasAttribute('download')) return;
    // 拦截所有链接点击，强制在当前 iframe 内打开
    e.preventDefault();
    e.stopPropagation();
    try{
      var url = new URL(href, location.href);
      // 同站链接：直接在当前 iframe 内导航
      if(url.origin === location.origin){
        location.href = url.href;
      } else {
        // 跨域链接：在当前 iframe 内打开（不跳出编辑界面）
        location.href = url.href;
      }
    }catch(err){
      // 无法解析的 href，放行
    }
  }, true);

  // Esc 取消选择模式
  document.addEventListener('keydown', function(e){
    if(pickMode && e.key === 'Escape'){
      disablePick();
      send('pick-cancelled', {});
    }
  }, true);

  // 页面加载完成上报
  send('page-loaded', { url: location.href, title: document.title });

  // 文字选区捕获（非选择模式下生效）
  document.addEventListener('mouseup', function(){
    if(pickMode) return;
    setTimeout(function(){
      var sel = window.getSelection();
      if(!sel || sel.isCollapsed) return;
      var range = sel.getRangeAt(0);
      var html = '';
      try{
        var div = document.createElement('div');
        div.appendChild(range.cloneContents());
        html = div.innerHTML;
      }catch(e){}
      if(!html || html.length < 1) return;
      send('selection', { html: html, xpath: describeEl(range.commonAncestorContainer.nodeType===3?range.commonAncestorContainer.parentNode:range.commonAncestorContainer) });
    }, 50);
  });

  // 监听父页面指令
  window.addEventListener('message', function(e){
    if(e.origin !== PARENT_ORIGIN) return;
    var msg = e.data || {};
    if(msg.type === 'enable-pick'){
      enablePick();
    } else if(msg.type === 'disable-pick'){
      disablePick();
    } else if(msg.type === 'reload'){
      location.reload();
    } else if(msg.type === 'navigate'){
      if(msg.url) location.href = msg.url;
    }
  });
})();`;

const DesignPreview: React.FC = () => {
  const intl = useIntl();
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [iframeUrl, setIframeUrl] = useState('');
  const [iframeTitle, setIframeTitle] = useState('');
  const [selectedDom, setSelectedDom] = useState('');
  const [selectedXPath, setSelectedXPath] = useState('');
  const [aiChatVisible, setAiChatVisible] = useState(true);
  const [device, setDevice] = useState<'pc' | 'mobile'>('pc');
  const [templateType, setTemplateType] = useState(1); // 1=自适应, 2=PC+移动分离
  const [iframeKey, setIframeKey] = useState(0);
  const [pickingMode, setPickingMode] = useState(false); // 是否处于元素选择模式
  const [canGoBack, setCanGoBack] = useState(false);

  const { initialState } = useModel('@@initialState');
  const baseUrl = useMemo(() => {
    return initialState?.system?.base_url || '';
  }, [initialState?.system?.base_url]);
  const mobileUrl = useMemo(() => {
    return initialState?.system?.mobile_url || '';
  }, [initialState?.system?.mobile_url]);

  // 加载站点设计信息，判断是否是 PC+移动分离模式
  useEffect(() => {
    getDesignInfo({})
      .then((res: any) => {
        if (res?.data) {
          setTemplateType(res.data.template_type || 1);
        }
      })
      .catch(() => {});
  }, []);

  // 构造 iframe 初始 URL
  useEffect(() => {
    if (!baseUrl) return;
    let url = baseUrl;
    // 如果是 PC+移动分离模式，且当前是 mobile，则用 mobile url
    if (templateType === 2 && device === 'mobile') {
      url = mobileUrl || baseUrl;
    }
    setIframeUrl(url);
    setIframeKey((k) => k + 1);
  }, [baseUrl, templateType, device]);

  // 监听 iframe postMessage
  useEffect(() => {
    const handler = (event: MessageEvent) => {
      // 校验 origin（安全要求）
      if (event.origin !== window.location.origin && event.origin !== baseUrl) {
        return;
      }
      const { type, url, html, xpath, title } = event.data || {};
      if (type === 'page-loaded') {
        setIframeUrl(url || '');
        setIframeTitle(title || '');
      } else if (type === 'pick-element') {
        // 元素选择器返回选中的元素
        setPickingMode(false);
        setSelectedDom(html || '');
        setSelectedXPath(xpath || '');
      } else if (type === 'pick-cancelled') {
        // 用户按 Esc 取消选择
        setPickingMode(false);
      } else if (type === 'selection') {
        setSelectedDom(html || '');
        setSelectedXPath(xpath || '');
      } else if (type === 'navigation') {
        // iframe 内链接跳转，更新 iframe src
        if (url && iframeRef.current) {
          iframeRef.current.src = url;
        }
      }
    };
    window.addEventListener('message', handler);
    return () => window.removeEventListener('message', handler);
  }, [baseUrl]);

  // 更新前进/后退按钮的可用状态
  const updateNavState = () => {
    try {
      const win = iframeRef.current?.contentWindow;
      if (win && win.history) {
        const len = win.history.length;
        setCanGoBack(len > 1);
      }
    } catch (e) {}
  };

  // iframe load 后注入 agent.js
  const handleIframeLoad = () => {
    const iframe = iframeRef.current;
    if (!iframe || !iframe.contentDocument) return;
    // 更新前进/后退按钮状态
    updateNavState();
    try {
      const script = iframe.contentDocument.createElement('script');
      script.textContent = AGENT_JS_CODE;
      iframe.contentDocument.body.appendChild(script);
    } catch (e) {
      // 跨域时无法注入，需要 iframe 内页面主动加载 agent.js
    }
  };

  // 手动刷新 iframe
  const handleRefresh = () => {
    setIframeKey((k) => k + 1);
  };

  // 后退：调用 iframe 的 history.back
  const handleGoBack = () => {
    try {
      iframeRef.current?.contentWindow?.history.back();
    } catch (e) {}
  };

  // 切换设备模式
  const handleDeviceChange = (newDevice: 'pc' | 'mobile') => {
    if (templateType !== 2) return;
    setDevice(newDevice);
  };

  // 在新窗口打开当前 iframe URL
  const handleOpenInNewTab = () => {
    if (iframeUrl) {
      window.open(iframeUrl, '_blank');
    }
  };

  // AI 修改模板后刷新 iframe，并清除选中状态
  const handleIframeReload = () => {
    setIframeKey((k) => k + 1);
    setSelectedDom('');
    setSelectedXPath('');
    message.success(intl.formatMessage({ id: 'design.preview.refresh-after-ai' }));
  };

  // 启用元素选择模式：向 iframe 发送 enable-pick 指令
  const handleEnablePick = () => {
    if (!iframeRef.current || !iframeRef.current.contentWindow) return;
    setPickingMode(true);
    iframeRef.current.contentWindow.postMessage(
      { type: 'enable-pick' },
      window.location.origin,
    );
    message.info(intl.formatMessage({ id: 'design.preview.pick-hint' }));
  };

  // 取消元素选择模式
  const handleDisablePick = () => {
    if (!iframeRef.current || !iframeRef.current.contentWindow) return;
    setPickingMode(false);
    iframeRef.current.contentWindow.postMessage(
      { type: 'disable-pick' },
      window.location.origin,
    );
  };

  // 清除选中元素
  const handleClearSelection = () => {
    setSelectedDom('');
    setSelectedXPath('');
    // 同时通知 iframe 清除持久高亮边框（通过 reload 实现）
    setIframeKey((k) => k + 1);
  };

  return (
    <div className="design-preview">
      {/* 顶部通栏 */}
      <div className="preview-toolbar">
        <div className="toolbar-left">
          <Space>
            <Tooltip title={intl.formatMessage({ id: 'design.preview.back' })}>
              <Button
                type="text"
                icon={<ArrowLeftOutlined />}
                disabled={!canGoBack}
                onClick={handleGoBack}
              />
            </Tooltip>
            <Tooltip
              title={intl.formatMessage({ id: 'design.preview.refresh' })}
            >
              <Button
                type="text"
                icon={<ReloadOutlined />}
                onClick={handleRefresh}
              />
            </Tooltip>
            <Tooltip
              title={intl.formatMessage({ id: 'design.preview.open-new-tab' })}
            >
              <Button
                type="text"
                icon={<GlobalOutlined />}
                onClick={handleOpenInNewTab}
              />
            </Tooltip>
            {/* PC/移动端切换（仅 PC+移动分离模式显示） */}
            {templateType === 2 && (
              <Space size={4}>
                <Button
                  type={device === 'pc' ? 'primary' : 'text'}
                  size="small"
                  icon={<DesktopOutlined />}
                  onClick={() => handleDeviceChange('pc')}
                >
                  PC
                </Button>
                <Button
                  type={device === 'mobile' ? 'primary' : 'text'}
                  size="small"
                  icon={<MobileOutlined />}
                  onClick={() => handleDeviceChange('mobile')}
                >
                  <FormattedMessage id="design.preview.mobile" />
                </Button>
              </Space>
            )}
            <Button
              type={aiChatVisible ? 'primary' : 'default'}
              icon={<RobotOutlined />}
              onClick={() => setAiChatVisible(!aiChatVisible)}
            >
              <FormattedMessage id="design.preview.ai-assistant" />
            </Button>
            {pickingMode ? (
              <Button
                type="primary"
                danger
                icon={<AimOutlined />}
                onClick={handleDisablePick}
              >
                <FormattedMessage id="design.preview.cancel-pick" />
              </Button>
            ) : (
              <Tooltip
                title={intl.formatMessage({
                  id: 'design.preview.pick-element-tip',
                })}
              >
                <Button
                  type="default"
                  icon={<AimOutlined />}
                  onClick={handleEnablePick}
                />
              </Tooltip>
            )}
          </Space>
        </div>
        <div className="toolbar-center">
          <Space size={8}>
            <Tag color="blue">
              {iframeTitle || intl.formatMessage({ id: 'design.preview.loading' })}
            </Tag>
            <span className="url-display" title={iframeUrl}>
              {iframeUrl}
            </span>
          </Space>
        </div>
        <div className="toolbar-right">
          <Space size={8}>
            {selectedDom && (
              <Tag
                color="orange"
                title={selectedXPath}
                closable
                onClose={(e) => {
                  e.preventDefault();
                  handleClearSelection();
                }}
              >
                <FormattedMessage id="design.preview.selected" />
                {selectedDom.slice(0, 30)}
                {selectedDom.length > 30 ? '...' : ''}
              </Tag>
            )}
          </Space>
        </div>
      </div>

      {/* iframe 预览区 */}
      <div className="preview-content">
        <div
          className="iframe-container"
          style={
            device === 'mobile'
              ? {
                  width: 375,
                  margin: '0 auto',
                  border: '1px solid #d9d9d9',
                  borderRadius: 4,
                }
              : { width: '100%', border: 'none' }
          }
        >
          <iframe
            key={iframeKey}
            ref={iframeRef}
            src={iframeUrl}
            onLoad={handleIframeLoad}
            style={{ width: '100%', height: '100%', border: 'none' }}
            title={intl.formatMessage({ id: 'design.preview.iframe-title' })}
          />
        </div>
      </div>

      {/* AI 对话浮窗（沿用现有 aiChat 组件的浮动窗口布局） */}
      <AiChat
        visible={aiChatVisible}
        onClose={() => setAiChatVisible(false)}
        iframeUrl={iframeUrl}
        selectedDom={selectedDom}
        onIframeReload={handleIframeReload}
      />
    </div>
  );
};

export default DesignPreview;
