export type BootstrapState =
  | 'UNINITIALIZED'
  | 'PREFLIGHT_CHECK'
  | 'ADMIN_CREATE'
  | 'DOMAIN_VERIFY'
  | 'TLS_PROVISION'
  | 'PANEL_HTTPS_ENABLE'
  | 'CORE_INSTALL'
  | 'CORE_CONFIG_RENDER'
  | 'SERVICE_PROFILE_CREATE'
  | 'SUBSCRIPTION_CREATE'
  | 'SECURITY_HARDEN'
  | 'FINAL_HEALTH_CHECK'
  | 'READY';

export type Locale = 'zh-CN' | 'en-US';

export function formatBootstrapState(state: BootstrapState, locale: Locale = 'zh-CN'): string {
  const zh: Record<BootstrapState, string> = {
    UNINITIALIZED: '待初始化',
    PREFLIGHT_CHECK: '预检查',
    ADMIN_CREATE: '创建管理员',
    DOMAIN_VERIFY: '域名与端口',
    TLS_PROVISION: '证书状态',
    PANEL_HTTPS_ENABLE: 'HTTPS 入口',
    CORE_INSTALL: '安装接入核心',
    CORE_CONFIG_RENDER: '生成配置',
    SERVICE_PROFILE_CREATE: '创建服务模板',
    SUBSCRIPTION_CREATE: '创建配置分发',
    SECURITY_HARDEN: '安全加固',
    FINAL_HEALTH_CHECK: '最终健康检查',
    READY: '就绪'
  };
  const en: Record<BootstrapState, string> = {
    UNINITIALIZED: 'Not initialized',
    PREFLIGHT_CHECK: 'Preflight checks',
    ADMIN_CREATE: 'Create administrator',
    DOMAIN_VERIFY: 'Domain and ports',
    TLS_PROVISION: 'Certificate status',
    PANEL_HTTPS_ENABLE: 'HTTPS entry',
    CORE_INSTALL: 'Install access core',
    CORE_CONFIG_RENDER: 'Render configuration',
    SERVICE_PROFILE_CREATE: 'Create service templates',
    SUBSCRIPTION_CREATE: 'Create distribution',
    SECURITY_HARDEN: 'Security hardening',
    FINAL_HEALTH_CHECK: 'Final health check',
    READY: 'Ready'
  };

  return (locale === 'en-US' ? en : zh)[state] ?? state;
}
