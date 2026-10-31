import CollapseItem from '@/components/collaspeItem';
import { getDesignTplHelpers } from '@/services';
import {
  deleteDesignHistoryFile,
  getDesignFileHistories,
  getDesignFileHistoryInfo,
  getDesignFileInfo,
  getDesignInfo,
  restoreDesignFileInfo,
  saveDesignFileInfo,
} from '@/services/design';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import {
  Button,
  Card,
  Col,
  Input,
  Modal,
  Popover,
  Row,
  Space,
  Tabs,
  Tree,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import MonacoEditor, { monaco } from 'react-monaco-editor';
import TemplateCompare from './components/compare';
import './index.less';

let fileType: string = '';
let helperEvent: any;

interface TabItem {
  key: string;
  path: string;
  type: string;
  label: string;
}

interface TabData {
  code: string;
  loaded: boolean;
  fileInfo: any;
  unsave: boolean;
}

const DesignEditor: React.FC = () => {
  const [fileInfo, setFileInfo] = useState<any>({});
  const [designInfo, setDesignInfo] = useState<any>({});
  const [showHistory, setShowHistory] = useState<boolean>(false);
  const [code, setCode] = useState<string>(``);
  const actionRef = useRef<ActionType>();
  const [loaded, setLoaded] = useState<boolean>(false);
  const [height, setHeight] = useState(0);
  const [tplFiles, setTplFiles] = useState<any[]>([]);
  const [staticFiles, setStaticFiles] = useState<any[]>([]);
  const [showDiff, setShowDiff] = useState<boolean>(false);
  const [historyContent, setHistoryContent] = useState<string>('');
  const [tplSelect, setTplSelect] = useState<React.Key[]>([]);
  const [staticSelect, setStaticSelect] = useState<React.Key[]>([]);
  const [showTplHelper, setShowTplHelper] = useState<boolean>(false);
  const [tplHelpers, setTplHelpers] = useState<any[]>();
  const [addCodeVisible, setAddCodeVisible] = useState<boolean>(false);
  const [addCode, setAddCode] = useState<any>({});
  const [codeValue, setCodeValue] = useState<string>('');
  const [tabs, setTabs] = useState<TabItem[]>([]);
  const [activeTab, setActiveTab] = useState<string>('');
  const [renderTick, setRenderTick] = useState(0);
  const tabDataRef = useRef<Record<string, TabData>>({});
  // 每个 tab 独立的 monaco model，持有各自的滚动位置、undo 栈、语言
  const modelsRef = useRef<Record<string, any>>({});
  // 每个 tab 独立的 view state（光标、滚动等），在切走前保存
  const viewStatesRef = useRef<Record<string, any>>({});
  const editorRef = useRef<any>(null);
  const intl = useIntl();

  const forceRender = () => setRenderTick((t) => t + 1);

  const getLanguage = (filePath: string) => {
    return filePath.indexOf('.html') !== -1
      ? 'html'
      : filePath.indexOf('.css') !== -1
      ? 'css'
      : filePath.indexOf('.yml') !== -1
      ? 'yaml'
      : 'javascript';
  };

  // 为指定 tab 创建（或复用）独立的 monaco model 并切到该 model
  const switchToTabModel = (
    key: string,
    fileInfoData: any,
    codeStr: string,
  ) => {
    const editor = editorRef.current;
    if (!editor || !key) return;
    const lang = getLanguage(fileInfoData?.path || '');
    // 用唯一 URI 创建 model，避免不同 tab/语言复用同一 in-memory model
    // 导致 css 颜色方块等装饰器残留到 html tab
    let model = modelsRef.current[key];
    if (!model) {
      const uri = monaco.Uri.parse(
        `inmemory://design/${encodeURIComponent(key)}`,
      );
      // 组件卸载时 model 可能仍留在 monaco 全局注册表中（如 HMR 后重挂载），
      // 创建前优先复用同 URI 的已有 model，避免 ModelService 重复注册报错
      model = monaco.editor.getModel(uri);
      if (!model) {
        model = monaco.editor.createModel(codeStr || '', lang, uri);
      }
      modelsRef.current[key] = model;
    } else {
      // model 已存在，只在内容真正变化时更新（避免覆盖用户编辑）
      if (model.getValue() !== (codeStr || '')) {
        model.setValue(codeStr || '');
      }
    }
    // 切走前保存当前 tab 的 view state（滚动位置/光标）
    const prevKey = activeTab;
    if (prevKey && prevKey !== key && editor.getModel()) {
      viewStatesRef.current[prevKey] = editor.saveViewState();
    }
    editor.setModel(model);
    // 恢复目标 tab 的 view state（各自的滚动位置）
    if (viewStatesRef.current[key]) {
      editor.restoreViewState(viewStatesRef.current[key]);
    }
  };

  const saveActiveTabData = () => {
    if (activeTab && tabDataRef.current[activeTab]) {
      tabDataRef.current[activeTab].code = code;
      tabDataRef.current[activeTab].loaded = loaded;
      tabDataRef.current[activeTab].fileInfo = { ...fileInfo };
      tabDataRef.current[activeTab].unsave =
        tabDataRef.current[activeTab].unsave || false;
    }
  };

  const restoreActiveTabData = (key: string) => {
    const data = tabDataRef.current[key];
    if (data) {
      setCode(data.code);
      setFileInfo({ ...data.fileInfo });
      setLoaded(data.loaded);
    } else {
      setCode('');
      setFileInfo({});
      setLoaded(false);
    }
  };

  const fetchDesignFileInfo = async (path: any, tabKey?: string) => {
    const searchParams = new URLSearchParams(window.location.search);
    const packageName = searchParams.get('package') || '';
    const key = tabKey || path;
    // Mark tab as loading
    if (tabDataRef.current[key]) {
      tabDataRef.current[key].loaded = false;
    }
    setLoaded(false);
    getDesignFileInfo({
      package: packageName,
      type: fileType,
      path: path,
    })
      .then((res) => {
        setFileInfo(res.data);
        setCode(res.data.content || '');
        setLoaded(true);
        // Store in tab data (always store when tabKey is provided)
        tabDataRef.current[key] = {
          code: res.data.content || '',
          loaded: true,
          fileInfo: { ...res.data },
          unsave: false,
        };
        // 切换到该 tab 的独立 model（保留各自滚动位置、避免跨语言色块残留）
        switchToTabModel(key, res.data, res.data.content || '');
        forceRender();
        actionRef.current?.reload();
      })
      .catch(() => {
        message.error(intl.formatMessage({ id: 'design.editor.get.error' }));
      });
  };

  const fetchDesignInfo = async () => {
    const searchParams = new URLSearchParams(window.location.search);
    const packageName = searchParams.get('package') || '';
    getDesignInfo({
      package: packageName,
    })
      .then((res) => {
        setDesignInfo(res.data);

        let path = searchParams.get('path') || '';
        let type = searchParams.get('type') || '';
        fileType = type + '';

        if (path === '' && res.data.tpl_files?.length > 0) {
          type = 'template';
          path = res.data.tpl_files[0].path;
        }
        let tpls: any[] = [];
        // eslint-disable-next-line guard-for-in
        for (let i in res.data.tpl_files) {
          let filePath = res.data.tpl_files[i].path;
          let remark = res.data.tpl_files[i].remark;
          let parts = filePath.split('/');
          if (parts.length > 1) {
            let firstLevelKey = parts[0];
            let secondLevelKey =
              parts.length > 2 ? parts[0] + '/' + parts[1] : null;
            let firstLevelNode = tpls.find(
              (item) => item.key === firstLevelKey,
            );
            if (!firstLevelNode) {
              firstLevelNode = {
                key: firstLevelKey,
                title: firstLevelKey,
                children: [],
              };
              tpls.push(firstLevelNode);
            }
            if (secondLevelKey && parts.length > 2) {
              let secondLevelNode = firstLevelNode.children.find(
                (item: any) => item.key === secondLevelKey,
              );

              if (!secondLevelNode) {
                secondLevelNode = {
                  key: secondLevelKey,
                  title: parts[1],
                  children: [],
                };
                firstLevelNode.children.push(secondLevelNode);
              }
              secondLevelNode.children.push({
                key: filePath,
                path: filePath,
                remark: remark,
                title: (
                  <div>
                    <div className="name">{parts.slice(2).join('/')}</div>
                    <div className="extra">{remark}</div>
                  </div>
                ),
              });
            } else {
              firstLevelNode.children.push({
                key: filePath,
                path: filePath,
                remark: remark,
                title: (
                  <div>
                    <div className="name">{parts.slice(1).join('/')}</div>
                    <div className="extra">{remark}</div>
                  </div>
                ),
              });
            }
          } else {
            tpls.push({
              path: filePath,
              remark: remark,
              title: (
                <div>
                  <div className="name">{filePath}</div>
                  <div className="extra">{remark}</div>
                </div>
              ),
              key: i,
            });
          }
        }
        setTplFiles(tpls);
        // static
        let statics: any[] = [];
        for (let i in res.data.static_files) {
          if (
            res.data.static_files[i].path.indexOf('.js') !== -1 ||
            res.data.static_files[i].path.indexOf('.ts') !== -1 ||
            res.data.static_files[i].path.indexOf('.css') !== -1 ||
            res.data.static_files[i].path.indexOf('.scss') !== -1 ||
            res.data.static_files[i].path.indexOf('.sass') !== -1 ||
            res.data.static_files[i].path.indexOf('.less') !== -1
          ) {
            let filePath = res.data.static_files[i].path;
            let remark = res.data.static_files[i].remark;
            let parts = filePath.split('/');
            if (parts.length > 1) {
              let firstLevelKey = parts[0];
              let secondLevelKey =
                parts.length > 2 ? parts[0] + '/' + parts[1] : null;
              let firstLevelNode = statics.find(
                (item) => item.key === firstLevelKey,
              );
              if (!firstLevelNode) {
                firstLevelNode = {
                  key: firstLevelKey,
                  title: firstLevelKey,
                  children: [],
                };
                statics.push(firstLevelNode);
              }
              if (secondLevelKey && parts.length > 2) {
                let secondLevelNode = firstLevelNode.children.find(
                  (item: any) => item.key === secondLevelKey,
                );

                if (!secondLevelNode) {
                  secondLevelNode = {
                    key: secondLevelKey,
                    title: parts[1],
                    children: [],
                  };
                  firstLevelNode.children.push(secondLevelNode);
                }
                secondLevelNode.children.push({
                  key: filePath,
                  path: filePath,
                  remark: remark,
                  title: (
                    <div>
                      <div className="name">{parts.slice(2).join('/')}</div>
                      <div className="extra">{remark}</div>
                    </div>
                  ),
                });
              } else {
                firstLevelNode.children.push({
                  key: filePath,
                  path: filePath,
                  remark: remark,
                  title: (
                    <div>
                      <div className="name">{parts.slice(1).join('/')}</div>
                      <div className="extra">{remark}</div>
                    </div>
                  ),
                });
              }
            } else {
              statics.push({
                path: filePath,
                remark: remark,
                title: (
                  <div>
                    <div className="name">{filePath}</div>
                    <div className="extra">{remark}</div>
                  </div>
                ),
                key: i,
              });
            }
          }
        }
        setStaticFiles(statics);

        // Open initial file as first tab
        const label = path.split('/').pop() || path;
        const tabKey = path;
        setTabs([{ key: tabKey, path, type, label }]);
        setActiveTab(tabKey);
        fileType = type;
        fetchDesignFileInfo(path, tabKey);
      })
      .catch(() => {
        message.error(intl.formatMessage({ id: 'design.editor.get.error' }));
      });
  };

  const getHeight = () => {
    let num = window?.innerHeight - 260;
    if (num < 450) {
      num = 450;
    } else if (num > 900) {
      num = 900;
    }

    setHeight(num);
  };

  useEffect(() => {
    fetchDesignInfo();
    getHeight();
    window.addEventListener('resize', getHeight);
    return () => {
      // 组件销毁时移除监听事件
      window.removeEventListener('resize', getHeight);
      // 组件销毁时释放所有 tab 的 monaco model（留在全局注册表会导致
      // 下次挂载 createModel 时报 ModelService: Cannot add model ...）
      Object.keys(modelsRef.current).forEach((key) => {
        try {
          modelsRef.current[key]?.dispose();
        } catch {
          // ignore
        }
        delete modelsRef.current[key];
      });
    };
  }, []);

  const getTplHelpers = () => {
    if (!tplHelpers) {
      getDesignTplHelpers().then((res: any) => {
        setTplHelpers(res.data || null);
      });
    }
  };

  const editorDidMount = (editor: any) => {
    editorRef.current = editor;
    editor.createContextKey('showTplHelperAction', true);
    editor.addAction({
      // id
      id: 'tpl-helper',
      // 该菜单键显示文本
      label: intl.formatMessage({ id: 'design.editor.helper' }),
      // 控制该菜单键显示
      precondition: 'showTplHelperAction',
      // 该菜单键位置
      contextMenuGroupId: 'navigation',
      contextMenuOrder: 1.5,
      // 点击该菜单键后运行
      run: (event: any) => {
        // 光标位置
        helperEvent = event;
        setShowTplHelper(true);
        getTplHelpers();
      },
    });
    // 如果挂载时已有 active tab 的内容，切换到对应 model
    if (activeTab && tabDataRef.current[activeTab]) {
      switchToTabModel(activeTab, tabDataRef.current[activeTab].fileInfo, code);
    }
  };

  const onChangeCode = (newCode: string) => {
    if (code !== newCode) {
      setCode(newCode);
      if (activeTab && tabDataRef.current[activeTab]) {
        tabDataRef.current[activeTab].unsave = true;
        tabDataRef.current[activeTab].code = newCode;
        forceRender();
      }
    }
  };

  const handleSave = () => {
    fileInfo.content = code;
    fileInfo.package = designInfo.package;
    fileInfo.update_content = true;
    fileInfo.type = fileType;
    // Update tab data
    if (activeTab && tabDataRef.current[activeTab]) {
      tabDataRef.current[activeTab].unsave = false;
      forceRender();
    }
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveDesignFileInfo(fileInfo)
      .then((res) => {
        message.info(res.msg);
        // Update tab data loaded status
        if (activeTab && tabDataRef.current[activeTab]) {
          tabDataRef.current[activeTab].fileInfo = { ...fileInfo };
        }
        actionRef.current?.reload();
      })
      .finally(() => {
        hide();
      });
  };

  const scrollToTop = () => {
    window.scrollTo(window.pageXOffset, 0);
  };

  const openNewTab = (type: string, info: any, key: string) => {
    // Save current tab data
    saveActiveTabData();
    // Add new tab
    const label = info.path.split('/').pop() || info.path;
    setTabs((prev) => [...prev, { key, path: info.path, type, label }]);
    setActiveTab(key);
    fileType = type;
    fetchDesignFileInfo(info.path, key);
    scrollToTop();
  };

  const handleTabChange = (key: string) => {
    if (key === activeTab) return;
    // Save current tab data
    saveActiveTabData();
    // Switch to new tab
    setActiveTab(key);
    const tab = tabs.find((t) => t.key === key);
    if (tab) {
      fileType = tab.type;
      restoreActiveTabData(key);
      // 切到目标 tab 的独立 model（恢复该 tab 各自的滚动位置/光标）
      const data = tabDataRef.current[key];
      if (data) {
        switchToTabModel(key, data.fileInfo, data.code);
      }
      scrollToTop();
    }
  };

  const handleEditFile = (type: string, info: any) => {
    const key = info.path;
    // Check if tab already open
    const existingIndex = tabs.findIndex((t) => t.key === key);
    if (existingIndex !== -1) {
      // Tab already open, switch to it
      handleTabChange(key);
      scrollToTop();
      return;
    }

    // Check if current tab has unsaved changes
    const currentTabData = activeTab ? tabDataRef.current[activeTab] : null;
    if (currentTabData?.unsave) {
      Modal.confirm({
        title: intl.formatMessage({ id: 'design.editor.confirm-giveup' }),
        content: intl.formatMessage({
          id: 'design.editor.confirm-giveup.content',
        }),
        onOk: () => {
          openNewTab(type, info, key);
        },
      });
    } else {
      openNewTab(type, info, key);
    }
  };

  const doRemoveTab = (targetKey: string) => {
    const remainingTabs = tabs.filter((t) => t.key !== targetKey);
    // Clean up tab data
    delete tabDataRef.current[targetKey];
    // 释放被关闭 tab 的独立 model，避免内存泄漏与脏装饰器残留
    const model = modelsRef.current[targetKey];
    if (model) {
      model.dispose();
      delete modelsRef.current[targetKey];
    }
    delete viewStatesRef.current[targetKey];

    if (remainingTabs.length === 0) {
      setTabs([]);
      setActiveTab('');
      setCode('');
      setFileInfo({});
      setLoaded(false);
      return;
    }

    // Determine next active tab
    let nextKey = activeTab;
    if (targetKey === activeTab) {
      const removedIndex = tabs.findIndex((t) => t.key === targetKey);
      const nextTab =
        remainingTabs[Math.min(removedIndex, remainingTabs.length - 1)];
      nextKey = nextTab.key;
    }

    setTabs(remainingTabs);
    setActiveTab(nextKey);
    const nextTab = remainingTabs.find((t) => t.key === nextKey);
    if (nextTab) {
      fileType = nextTab.type;
      restoreActiveTabData(nextKey);
      // 切到下一个 tab 的独立 model（恢复其滚动位置/光标）
      const data = tabDataRef.current[nextKey];
      if (data) {
        switchToTabModel(nextKey, data.fileInfo, data.code);
      }
    }
  };

  const handleTabRemove = (targetKey: string) => {
    const tab = tabs.find((t) => t.key === targetKey);
    if (!tab) return;

    const tabData = tabDataRef.current[targetKey];
    if (tabData?.unsave) {
      Modal.confirm({
        title: intl.formatMessage({ id: 'design.editor.confirm-giveup' }),
        content: intl.formatMessage({
          id: 'design.editor.confirm-giveup.content',
        }),
        onOk: () => {
          doRemoveTab(targetKey);
        },
      });
    } else {
      doRemoveTab(targetKey);
    }
  };

  const handleCompare = (info: any) => {
    getDesignFileHistoryInfo({
      hash: info.hash,
      package: designInfo.package,
      path: fileInfo.path,
      type: fileType,
    }).then((res) => {
      if (res.code !== 0) {
        message.error(res.msg);
      } else {
        setHistoryContent(res.data);
        setShowDiff(true);
      }
    });
  };

  const handleRestore = (info: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.editor.confirm-restore' }),
      content: intl.formatMessage({
        id: 'design.editor.confirm-restore.content',
      }),
      onOk: () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        restoreDesignFileInfo({
          hash: info.hash,
          package: designInfo.package,
          path: fileInfo.path,
          type: fileType,
        })
          .then((res) => {
            message.info(res.msg);
            fetchDesignFileInfo(info.path, activeTab);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const deleteHistoryFile = (info: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.editor.history.confirm-delete' }),
      onOk: () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        deleteDesignHistoryFile({
          hash: info.hash,
          package: designInfo.package,
          path: fileInfo.path,
          type: fileType,
        })
          .then((res) => {
            message.info(res.msg);
            actionRef.current?.reload();
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 's' && (event.ctrlKey || event.metaKey)) {
      // 自动保存
      handleSave();

      event.preventDefault();
    }
  };

  const handleGoBack = () => {
    const currentTabData = activeTab ? tabDataRef.current[activeTab] : null;
    if (currentTabData?.unsave) {
      Modal.confirm({
        title: intl.formatMessage({ id: 'design.editor.confirm-goback' }),
        content: intl.formatMessage({
          id: 'design.editor.confirm-giveup.content',
        }),
        onOk: () => {
          history.back();
        },
      });
    } else {
      history.back();
    }
  };

  const getSize = (size: any) => {
    if (size < 500) {
      return size + 'B';
    }
    if (size < 1024 * 1024) {
      return (size / 1024).toFixed(2) + 'KB';
    }

    return (size / 1024 / 1024).toFixed(2) + 'MB';
  };

  const handleAddCode = (addCode: any, docLink: string) => {
    if (docLink) {
      addCode.link = docLink;
    }
    setAddCode(addCode);
    setCodeValue(addCode.code);
    setAddCodeVisible(true);
  };

  const onSubmitAddCode = () => {
    if (helperEvent) {
      let position = helperEvent.getPosition();
      helperEvent?.executeEdits('', [
        {
          range: new monaco.Range(
            position.lineNumber,
            position.column,
            position.lineNumber,
            position.column,
          ),
          text: codeValue,
        },
      ]);

      helperEvent.focus();
    }

    setAddCodeVisible(false);
    setShowTplHelper(false);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'Hash',
      dataIndex: 'hash',
    },
    {
      title: intl.formatMessage({ id: 'design.size' }),
      dataIndex: 'size',
      render: (text: any) => <div>{getSize(text)}</div>,
    },
    {
      title: intl.formatMessage({ id: 'design.update-time' }),
      dataIndex: 'last_mod',
      render: (text: any) =>
        dayjs((text as number) * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      key: 'action',
      width: 110,
      render: (_: any, record: any) => (
        <Space size={12}>
          <Button
            type="link"
            onClick={() => {
              handleCompare(record);
            }}
          >
            <FormattedMessage id="design.compare" />
          </Button>
          <Button
            type="link"
            onClick={() => {
              handleRestore(record);
            }}
          >
            <FormattedMessage id="design.restore" />
          </Button>
          <Button
            danger
            type="link"
            onClick={() => {
              deleteHistoryFile(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title={
        <div>
          <FormattedMessage id="design.editing" />:{' '}
          {activeTab
            ? tabs.find((t) => t.key === activeTab)?.path || fileInfo?.path
            : fileInfo?.path}
        </div>
      }
    >
      <Card className="design-editor-card">
        <Row gutter={16}>
          <Col sm={18} xs={24}>
            {tabs.length > 0 && (
              <Tabs
                type="editable-card"
                hideAdd
                activeKey={activeTab}
                onChange={handleTabChange}
                onEdit={(targetKey) => handleTabRemove(targetKey as string)}
                size="small"
                items={tabs.map((tab) => ({
                  key: tab.key,
                  label: (
                    <span>
                      {tab.label}
                      {tabDataRef.current[tab.key]?.unsave && (
                        <span style={{ color: '#ff4d4f', marginLeft: 2 }}>
                          *
                        </span>
                      )}
                    </span>
                  ),
                }))}
                style={{ marginBottom: 8 }}
              />
            )}
            <div className="code-editor-box" onKeyDown={handleKeyDown}>
              {loaded && (
                <MonacoEditor
                  height={height}
                  theme="vs-dark"
                  options={{
                    selectOnLineNumbers: false,
                    wordWrap: 'on',
                  }}
                  onChange={onChangeCode}
                  editorDidMount={editorDidMount}
                />
              )}
            </div>
            <div className="mt-normal">
              <Space size={16}>
                <Button
                  type="primary"
                  onClick={() => {
                    handleSave();
                  }}
                >
                  <FormattedMessage id="design.save" />
                </Button>
                <Button onClick={handleGoBack}>
                  <FormattedMessage id="design.back" />
                </Button>
                <Button
                  onClick={() => {
                    setShowHistory(true);
                  }}
                >
                  <FormattedMessage id="design.history.view" />
                </Button>
                <div className="text-muted">
                  <FormattedMessage id="design.editor.tips" />
                </div>
              </Space>
            </div>
          </Col>
          <Col sm={6} xs={24}>
            <CollapseItem
              className="tpl-file-list"
              showArrow={false}
              open
              header={intl.formatMessage({ id: 'design.tempalte.name' })}
              key="1"
            >
              <Tree
                showLine={true}
                showIcon={false}
                expandedKeys={tplSelect}
                onSelect={(_, a) => {
                  if (a.node.children) {
                    let key = a.node.key;
                    let tmpItems = tplSelect.flat();
                    let index = tmpItems.indexOf(key);
                    if (index === -1) {
                      tmpItems.push(key);
                    } else {
                      tmpItems.splice(index, 1);
                    }
                    setTplSelect(tmpItems);
                    return;
                  }
                  handleEditFile('template', a.node);
                }}
                onExpand={(k) => {
                  setTplSelect(k);
                }}
                treeData={tplFiles}
              />
            </CollapseItem>
            <CollapseItem
              className="tpl-file-list"
              showArrow={false}
              header={intl.formatMessage({ id: 'design.static.name' })}
              key="2"
            >
              <Tree
                showLine={true}
                showIcon={false}
                expandedKeys={staticSelect}
                onSelect={(_, a) => {
                  if (a.node.children) {
                    let key = a.node.key;
                    let tmpItems = staticSelect.flat();
                    let index = tmpItems.indexOf(key);
                    if (index === -1) {
                      tmpItems.push(key);
                    } else {
                      tmpItems.splice(index, 1);
                    }
                    setStaticSelect(tmpItems);
                    return;
                  }
                  handleEditFile('static', a.node);
                }}
                onExpand={(k) => {
                  console.log(k);
                  setStaticSelect(k);
                }}
                treeData={staticFiles}
              />
            </CollapseItem>
          </Col>
        </Row>
      </Card>
      <Modal
        title={intl.formatMessage({ id: 'design.history.name' })}
        open={showHistory}
        onCancel={() => {
          setShowHistory(false);
        }}
        onOk={() => {
          setShowHistory(false);
        }}
        width={800}
      >
        <ProTable<any>
          headerTitle={intl.formatMessage({ id: 'design.manage.name' })}
          actionRef={actionRef}
          rowKey="path"
          search={false}
          toolBarRender={false}
          request={async (params) => {
            params.package = designInfo.package;
            params.path = fileInfo.path;
            return getDesignFileHistories(params);
          }}
          columnsState={{
            persistenceKey: 'design-file-history-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          pagination={{
            showSizeChanger: true,
          }}
        />
      </Modal>
      {showDiff && (
        <TemplateCompare
          open={showDiff}
          originCode={historyContent}
          language={getLanguage(fileInfo?.path || '')}
          versionCode={code}
          onCancel={() => setShowDiff(false)}
          onFinished={(e) => {
            onChangeCode(e);
            setShowDiff(false);
          }}
        />
      )}
      <Modal
        open={showTplHelper}
        onCancel={() => {
          setShowTplHelper(false);
        }}
        onOk={() => {
          setShowTplHelper(false);
        }}
        width={800}
      >
        {tplHelpers?.map((item: any, index: number) => {
          return (
            <div key={index} className="design-helper">
              <h3 className="helper-header">{item.title}</h3>
              <Row gutter={16} wrap>
                {item.docs?.map((doc: any, index2: number) => (
                  <Col key={index2}>
                    {doc.docs ? (
                      <Popover
                        content={
                          <div className="helper-popover">
                            <Row gutter={16} wrap>
                              {doc.docs.map((child: any, index3: number) => (
                                <Col key={index3}>
                                  <span
                                    className="popover-item link"
                                    onClick={() =>
                                      handleAddCode(
                                        child,
                                        child.link || doc.link,
                                      )
                                    }
                                  >
                                    {child.title}
                                  </span>
                                </Col>
                              ))}
                            </Row>
                          </div>
                        }
                        trigger="click"
                      >
                        <span className="helper-item">{doc.title}</span>
                      </Popover>
                    ) : (
                      <span
                        className="helper-item link"
                        onClick={() => handleAddCode(doc, doc.link)}
                      >
                        {doc.title}
                      </span>
                    )}
                  </Col>
                ))}
              </Row>
            </div>
          );
        })}
      </Modal>
      <Modal
        title={intl.formatMessage({ id: 'design.segment.name' })}
        open={addCodeVisible}
        onCancel={() => {
          setAddCodeVisible(false);
        }}
        okText={intl.formatMessage({ id: 'design.insert' })}
        onOk={onSubmitAddCode}
        width={600}
      >
        {(addCode.content || addCode.link) && (
          <div className="helper-code-desc">
            {addCode.content && (
              <span>
                <FormattedMessage id="design.explain" />: {addCode.content}
              </span>
            )}
            {addCode.link && (
              <a href={addCode.link} target="_blank" rel="noreferrer">
                <FormattedMessage id="design.view-doc" />
              </a>
            )}
          </div>
        )}
        {addCodeVisible && (
          <Input.TextArea
            placeholder={intl.formatMessage({ id: 'design.segment.name' })}
            rows={10}
            value={codeValue}
            onChange={(e) => setCodeValue(e.target.value)}
          />
        )}
      </Modal>
    </PageContainer>
  );
};

export default DesignEditor;
