import { DomEditor, IDomEditor, IButtonMenu } from '@wangeditor/core';

// 定义菜单 class
class HtmlMenu implements IButtonMenu {
  readonly title = '编辑源码';
  readonly iconSvg =
    '<svg viewBox="64 64 896 896" focusable="false" data-icon="right-square" width="1em" height="1em" fill="currentColor" aria-hidden="true"><path d="M412.7 696.5l246-178c4.4-3.2 4.4-9.7 0-12.9l-246-178c-5.3-3.8-12.7 0-12.7 6.5V381c0 10.2 4.9 19.9 13.2 25.9L558.6 512 413.2 617.2c-8.3 6-13.2 15.6-13.2 25.9V690c0 6.5 7.4 10.3 12.7 6.5z"></path><path d="M880 112H144c-17.7 0-32 14.3-32 32v736c0 17.7 14.3 32 32 32h736c17.7 0 32-14.3 32-32V144c0-17.7-14.3-32-32-32zm-40 728H184V184h656v656z"></path></svg>';

  readonly tag = 'button';
  alwaysEnable = true

  htmlMode = false;

  getValue(editor: IDomEditor): string | boolean {
    // 插入菜单，不需要 value
    return '';
  }
  isActive(editor: IDomEditor): boolean {
    // 任何时候，都不用激活 menu
    return false;
  }

  private getMenuConfig(editor: IDomEditor): any {
    // 获取配置，见 `./config.js`
    return editor.getMenuConfig('html')
  }

  async exec(editor: IDomEditor, value: string | boolean) {
    const { customHtml,setMode } = this.getMenuConfig(editor)
    if(setMode) {
      setMode(!this.htmlMode)
    }
    if (this.htmlMode) {
      editor.enable();
      this.htmlMode = false;
      return;
    }
    this.htmlMode = true;
    editor.disable();
    if (customHtml) {
      customHtml((html:string) => editor.setHtml(html))
      return
    }
  }

  isDisabled(editor: IDomEditor): boolean {
    return false;
  }
}

// 定义菜单配置
export const HtmlMenuConf = {
  key: 'html', // menu key ，唯一。注册之后，可配置到工具栏
  factory() {
    return new HtmlMenu();
  },
};
