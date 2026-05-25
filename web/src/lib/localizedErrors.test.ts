import { describe, expect, it } from 'vitest';
import { ApiError } from './api';
import { localizedErrorMessage } from './localizedErrors';

describe('localizedErrorMessage', () => {
  it('maps known service configuration errors in Chinese', () => {
    const message = localizedErrorMessage(
      new ApiError('Service configuration could not be written.', {
        code: 'SERVICE_CONFIG_WRITE_FAILED',
        status: 500
      }),
      'zh-CN',
      'fallback'
    );

    expect(message).toContain('服务配置文件无法写入');
  });

  it('keeps backend message in English mode', () => {
    const message = localizedErrorMessage(
      new ApiError('Service configuration could not be written.', {
        code: 'SERVICE_CONFIG_WRITE_FAILED',
        status: 500
      }),
      'en-US',
      'fallback'
    );

    expect(message).toBe('Service configuration could not be written.');
  });
});
