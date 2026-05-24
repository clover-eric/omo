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

export function formatBootstrapState(state: BootstrapState): string {
  const labels: Record<BootstrapState, string> = {
    UNINITIALIZED: '待初始化',
    PREFLIGHT_CHECK: '预检中',
    ADMIN_CREATE: '创建管理员',
    DOMAIN_VERIFY: '域名与端口',
    TLS_PROVISION: '证书状态',
    PANEL_HTTPS_ENABLE: 'HTTPS 入口',
    CORE_INSTALL: '安装接入核心',
    CORE_CONFIG_RENDER: '生成配置',
    SERVICE_PROFILE_CREATE: '创建服务画像',
    SUBSCRIPTION_CREATE: '创建智能订阅',
    SECURITY_HARDEN: '安全加固',
    FINAL_HEALTH_CHECK: '最终健康检查',
    READY: '就绪'
  };

  return labels[state];
}
