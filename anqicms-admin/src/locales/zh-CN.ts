import account from './zh-CN/account';
import ai from './zh-CN/ai';
import common from './zh-CN/common';
import component from './zh-CN/component';
import content from './zh-CN/content';
import dashboard from './zh-CN/dashboard';
import design from './zh-CN/design';
import menu from './zh-CN/menu';
import pages from './zh-CN/pages';
import plugin from './zh-CN/plugin';
import pwa from './zh-CN/pwa';
import setting from './zh-CN/setting';
import statistic from './zh-CN/statistic';
import tool from './zh-CN/tool';
import website from './zh-CN/website';

export default {
  'site.title': '安企CMS',
  ...pages,
  ...menu,
  ...setting,
  ...pwa,
  ...component,
  ...dashboard,
  ...tool,
  ...website,
  ...account,
  ...ai,
  ...statistic,
  ...design,
  ...content,
  ...plugin,
  ...common,
};
