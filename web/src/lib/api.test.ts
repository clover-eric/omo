import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, apiPost } from './api';

const okEnvelope = (data: unknown) =>
  Promise.resolve({
    ok: true,
    json: () =>
      Promise.resolve({
        success: true,
        data,
        error: null,
        requestId: 'req_test'
      })
  } as Response);

describe('apiPost', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('prepares a csrf cookie before the first state-changing request', async () => {
    vi.stubGlobal('document', { cookie: '' });
    const fetchMock = vi
      .fn()
      .mockImplementationOnce(() => {
        document.cookie = 'omo_csrf=test-token';
        return okEnvelope({ csrfReady: true });
      })
      .mockImplementationOnce(() => okEnvelope({ accepted: true }));
    vi.stubGlobal('fetch', fetchMock);

    const result = await apiPost<{ accepted: boolean }>('/api/auth/login', {
      username: 'admin',
      password: 'StrongPassw0rd!'
    });

    expect(result.accepted).toBe(true);
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/security/csrf', {
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json'
      }
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/auth/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        'X-CSRF-Token': 'test-token'
      },
      body: JSON.stringify({ username: 'admin', password: 'StrongPassw0rd!' })
    });
  });

  it('preserves api error codes for localized page handling', async () => {
    vi.stubGlobal('document', { cookie: 'omo_csrf=test-token' });
    vi.stubGlobal(
      'fetch',
      vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 500,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: () =>
            Promise.resolve({
              success: false,
              data: null,
              error: {
                code: 'SERVICE_CONFIG_WRITE_FAILED',
                message: 'Service configuration could not be written.',
                details: {}
              },
              requestId: 'req_test'
            })
        } as Response)
      )
    );

    await expect(apiPost('/api/services/standard-secure-access/apply', {})).rejects.toMatchObject({
      name: 'ApiError',
      code: 'SERVICE_CONFIG_WRITE_FAILED',
      status: 500
    } satisfies Partial<ApiError>);
  });
});
