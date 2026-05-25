import Activity from '@lucide/svelte/icons/activity';
import Boxes from '@lucide/svelte/icons/boxes';
import ClipboardList from '@lucide/svelte/icons/clipboard-list';
import Gauge from '@lucide/svelte/icons/gauge';
import Network from '@lucide/svelte/icons/network';
import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
import type { Language } from '$lib/preferences';

export type ConsoleCopyKey =
  | 'overview'
  | 'serviceLibrary'
  | 'distribution'
  | 'cascadeNodes'
  | 'serverCheckup'
  | 'auditLogs'
  | 'settings'
  | 'brandSubtitle'
  | 'operationsConsole'
  | 'interfacePreferences'
  | 'switchToChinese'
  | 'switchToEnglish'
  | 'switchToLight'
  | 'switchToDark'
  | 'currentLanguageShort'
  | 'nextLanguageShort';

export const consoleCopy: Record<Language, Record<ConsoleCopyKey, string>> = {
  'zh-CN': {
    overview: '概览',
    serviceLibrary: '服务库',
    distribution: '配置分发',
    cascadeNodes: '级联节点',
    serverCheckup: '服务器体检',
    auditLogs: '审计日志',
    settings: '设置',
    brandSubtitle: '边界运维',
    operationsConsole: '运维控制台',
    interfacePreferences: '界面偏好',
    switchToChinese: '切换为简体中文',
    switchToEnglish: '切换为英文',
    switchToLight: '切换为浅色模式',
    switchToDark: '切换为深色模式',
    currentLanguageShort: '中文',
    nextLanguageShort: 'EN'
  },
  'en-US': {
    overview: 'Overview',
    serviceLibrary: 'Service Library',
    distribution: 'Distribution',
    cascadeNodes: 'Cascade Nodes',
    serverCheckup: 'Server Checkup',
    auditLogs: 'Audit Logs',
    settings: 'Settings',
    brandSubtitle: 'Boundary Operations',
    operationsConsole: 'Operations Console',
    interfacePreferences: 'Interface preferences',
    switchToChinese: 'Switch to Chinese',
    switchToEnglish: 'Switch to English',
    switchToLight: 'Switch to light mode',
    switchToDark: 'Switch to dark mode',
    currentLanguageShort: 'English',
    nextLanguageShort: '中文'
  }
};

export const consoleNavItems = [
  { key: 'overview', icon: Gauge, href: '/dashboard' },
  { key: 'serviceLibrary', icon: Boxes, href: '/services' },
  { key: 'distribution', icon: ClipboardList, href: '/subscriptions' },
  { key: 'cascadeNodes', icon: Network, href: '/cascade' },
  { key: 'serverCheckup', icon: Activity, href: '/diagnostics' },
  { key: 'auditLogs', icon: ClipboardList, href: '/logs' },
  { key: 'settings', icon: SlidersHorizontal, href: '/settings' }
] satisfies Array<{ key: ConsoleCopyKey; icon: typeof Gauge; href: string }>;

export function consoleLabels(language: Language) {
  return consoleCopy[language];
}
