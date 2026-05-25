import { ApiError } from '$lib/api';
import type { Language } from '$lib/preferences';

const zhMessages: Record<string, string> = {
  NON_JSON_RESPONSE: '接口返回格式异常，请刷新页面或检查 OMO 服务状态。',
  SERVICE_CONFIG_FAILED: '服务配置应用失败，系统已尽量保留或恢复之前的配置。请确认安装脚本已更新，并检查 OMO 服务日志。',
  SERVICE_CONFIG_WRITE_FAILED: '服务配置文件无法写入。请确认 OMO 对 /var/lib/omo 具有写入权限后重试。',
  SERVICE_PROFILE_NOT_DISTRIBUTION_READY: '该服务方案尚未完成独立协议验证，当前不会导出到配置分发。请先使用标准安全接入。',
  SERVICE_PROFILE_NOT_FOUND: '未找到该服务模板。',
  SERVICE_ROLLBACK_UNAVAILABLE: '暂无可回滚的上一版服务配置。',
  SERVICE_CONFIG_UNAVAILABLE: '服务配置管理暂不可用。',
  INVALID_SERVICE_INPUT: '服务输入无效，请检查名称、端口和状态。',
  SERVICE_CREATE_FAILED: '无法创建托管服务实例。',
  SUBSCRIPTION_FAILED: '配置分发操作失败。',
  INVALID_SUBSCRIPTION_INPUT: '配置分发输入无效。',
  SUBSCRIPTION_NOT_FOUND: '未找到该配置分发记录。',
  CASCADE_OPERATION_FAILED: '级联操作失败，请稍后重试。',
  INVALID_CASCADE_INPUT: '级联输入无效。',
  PAIRING_CODE_NOT_FOUND: '配对码不可用或已过期。',
  CASCADE_PEER_EXCHANGE_FAILED: '对端级联交换失败，请确认对端 OMO HTTPS 入口后重试。',
  CASCADE_CONFIRMATION_REQUIRED: '应用级联配置前需要运维人员确认。',
  DIAGNOSTICS_RUN_FAILED: '服务器体检运行失败，请稍后重试。',
  SETTINGS_SAVE_FAILED: '设置保存失败。',
  UPDATE_MANIFEST_INVALID: '更新清单必须是有效的 HTTPS 地址。',
  UPDATE_CONFIRMATION_REQUIRED: '应用或回滚更新前需要确认。',
  BACKUP_OPERATION_FAILED: '备份操作失败，请稍后重试。'
};

export function localizedErrorMessage(error: unknown, language: Language, fallback: string): string {
  if (error instanceof ApiError) {
    if (language === 'zh-CN' && zhMessages[error.code]) {
      return zhMessages[error.code];
    }
    return error.message || fallback;
  }
  return error instanceof Error ? error.message : fallback;
}
