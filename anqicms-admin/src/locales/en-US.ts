import account from './en-US/account';
import ai from './en-US/ai';
import common from './en-US/common';
import component from './en-US/component';
import content from './en-US/content';
import dashboard from './en-US/dashboard';
import design from './en-US/design';
import menu from './en-US/menu';
import pages from './en-US/pages';
import plugin from './en-US/plugin';
import pwa from './en-US/pwa';
import setting from './en-US/setting';
import statistic from './en-US/statistic';
import tool from './en-US/tool';
import website from './en-US/website';

export default {
  'site.title': 'AnQiCMS',
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
