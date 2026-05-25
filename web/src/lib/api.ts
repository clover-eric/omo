export type Envelope<T> = {
  success: boolean;
  data: T | null;
  error: {
    code: string;
    message: string;
    details: Record<string, unknown>;
  } | null;
  requestId: string;
};

export class ApiError extends Error {
  code: string;
  details: Record<string, unknown>;
  status: number;

  constructor(message: string, options: { code?: string; details?: Record<string, unknown>; status?: number } = {}) {
    super(message);
    this.name = 'ApiError';
    this.code = options.code ?? 'REQUEST_FAILED';
    this.details = options.details ?? {};
    this.status = options.status ?? 0;
  }
}

export type BootstrapStatus = {
  state: string;
  initialized: boolean;
  phase1Complete: boolean;
  requiresToken: boolean;
  domain?: string;
  latestJob?: BootstrapJob;
  nextRequirement?: string;
};

export type BootstrapJob = {
  id: string;
  kind: string;
  state: string;
  status: string;
  progress: number;
  userMessage: string;
};

export type SingBoxStatus = {
  installed: boolean;
  healthy: boolean;
  version?: string;
  path?: string;
  installSource?: string;
  message: string;
};

export type SystemOverview = {
  status: 'ok';
  service: 'omo';
  version: string;
  timestamp: string;
  bootstrap?: BootstrapStatus;
  core: SingBoxStatus;
  counts?: {
    admins: number;
    serviceProfiles: number;
    services: number;
  };
};

export type ServiceProfile = {
  id: string;
  version: string;
  displayName: string;
  category: 'standard' | 'performance' | 'compatibility' | 'fallback' | 'mobility';
  summary: string;
  expertProtocol: string;
  transport: string;
  securityLayer: string;
  requiresDomain: boolean;
  requiresTLSCert: boolean;
  requiresUdp: boolean;
  defaultPortPolicy: string;
  dependencies: string[];
  clientFormats: string[];
  scoreWeights: Record<string, number>;
  templateRef: string;
  goldenTestRef: string;
  rollbackStrategy: string;
};

export type ServiceConfigJobResult = {
  job: BootstrapJob;
  result: {
    profileId: string;
    profileVersion?: string;
    profileDisplayName?: string;
    expertProtocol?: string;
    configVersion: string;
    configPath: string;
    backupPath?: string;
    rolledBack: boolean;
    listenPort: number;
  };
  instances?: ServiceInstance[];
};

export type ServiceInstance = {
  id: string;
  profileId: string;
  displayName: string;
  listenPort: number;
  status: 'planned' | 'active' | 'disabled';
  configVersion: string;
  createdAt: string;
  updatedAt: string;
};

export type ServiceList = {
  services: ServiceInstance[];
  profiles: ServiceProfile[];
};

export type ServiceResult = {
  service: ServiceInstance;
};

export type SubscriptionToken = {
  id: string;
  name: string;
  status: 'active' | 'disabled';
  expiresAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type SubscriptionList = {
  subscriptions: SubscriptionToken[];
};

export type SubscriptionTokenResult = {
  subscription: SubscriptionToken;
  token: string;
  url: string;
};

export type DiagnosticCheck = {
  id: string;
  label: string;
  status: 'ok' | 'warning' | 'error';
  message: string;
  evidence?: string;
};

export type SystemSnapshot = {
  hostname: string;
  os: string;
  architecture: string;
  cpuCount: number;
  goVersion: string;
  processId: number;
  memoryAllocMb: number;
  memorySystemMb: number;
};

export type DiagnosticReport = {
  id: string;
  status: 'ok' | 'warning' | 'error';
  summary: string;
  checks: DiagnosticCheck[];
  system: SystemSnapshot;
  createdAt: string;
};

export type DiagnosticsRunResult = {
  job: BootstrapJob;
  report: DiagnosticReport;
};

export type DiagnosticsLatestResult = {
  report: DiagnosticReport | null;
  latestJob?: BootstrapJob | null;
};

export type DiagnosticsExternalProviderSettings = {
  enabled: boolean;
  name: string;
  endpointUrl: string;
  timeoutSeconds: number;
  apiKeyConfigured: boolean;
};

export type SettingsResponse = {
  diagnosticsExternalProvider: DiagnosticsExternalProviderSettings;
  updateManifestUrl?: string;
};

export type BackupRecord = {
  id: string;
  status: 'running' | 'ready' | 'failed';
  path: string;
  checksum?: string;
  createdAt: string;
  completedAt?: string;
};

export type BackupListResult = {
  backups: BackupRecord[];
};

export type BackupCreateResult = {
  backup: BackupRecord;
  job: BootstrapJob;
};

export type BackupRestoreResult = {
  backup: BackupRecord;
  job: BootstrapJob;
  restored: boolean;
};

export type AuditLog = {
  id: string;
  actorAdminId?: string;
  action: string;
  resourceType: string;
  resourceId?: string;
  details: Record<string, unknown>;
  createdAt: string;
};

export type AuditListResult = {
  logs: AuditLog[];
};

export type UpdateCheckResult = {
  configured: boolean;
  currentVersion: string;
  latestVersion?: string;
  updateAvailable: boolean;
  channel?: string;
  summary: string;
  manifestUrl?: string;
  checksumSha256?: string;
  signature?: string;
  artifactUrl?: string;
  checkedAt: string;
  platform: string;
};

export type UpdateJobResult = {
  job: BootstrapJob;
  version?: string;
  previousVersion?: string;
  backupId?: string;
  applied: boolean;
  rolledBack: boolean;
  artifactUrl?: string;
  checksumSha256?: string;
};

export type PairingCode = {
  id: string;
  nodeId: string;
  nodeName: string;
  domain: string;
  status: 'active' | 'used' | 'revoked' | 'expired';
  expiresAt: string;
  createdAt: string;
};

export type PairingCodeResult = {
  pairing: PairingCode;
  code: string;
};

export type CascadeNode = {
  id: string;
  name: string;
  domain: string;
  status: 'pending' | 'trusted' | 'disabled';
  role: string;
  trustKeyFingerprint?: string;
  online: boolean;
  latencyMs: number;
  throughputMbps: number;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
};

export type CascadePair = {
  id: string;
  sourceNodeId: string;
  targetNodeId: string;
  status: string;
  configState: string;
  createdAt: string;
  updatedAt: string;
};

export type CascadeConfigPlan = {
  pairId: string;
  sourceNodeId: string;
  targetNodeId: string;
  version: string;
  generatedAt: string;
  configPath: string;
  backupPath?: string;
  warnings: string[];
  summary: string;
  preview: Record<string, unknown>;
};

export type CascadeConfigPlanResult = {
  pair: CascadePair;
  plan: CascadeConfigPlan;
};

export type CascadeConfigApplyResult = {
  pair: CascadePair;
  plan: CascadeConfigPlan;
  job: BootstrapJob;
};

export type CascadeHealthSample = {
  nodeId: string;
  status: 'online' | 'offline' | 'skipped';
  online: boolean;
  latencyMs: number;
  throughputMbps: number;
  lastError?: string;
  sampledAt: string;
};

export type CascadeHealthSampleResult = {
  nodes: CascadeNode[];
  pairs: CascadePair[];
  samples: CascadeHealthSample[];
  job: BootstrapJob;
};

export type CascadeNodeList = {
  nodes: CascadeNode[];
  pairs: CascadePair[];
};

export type PairingAcceptResult = {
  node: CascadeNode;
  pair: CascadePair;
  job: BootstrapJob;
};

export type BootstrapEvent = {
  id: number;
  jobId: string;
  kind: string;
  state: string;
  status: string;
  progress: number;
  message: string;
  errorCode?: string;
  createdAt: string;
};

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json'
    }
  });
  return parseEnvelope<T>(response);
}

export async function apiPost<T>(path: string, body: unknown): Promise<T> {
  await ensureCSRF();
  const response = await fetch(path, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken()
    },
    body: JSON.stringify(body)
  });
  return parseEnvelope<T>(response);
}

export async function apiPatch<T>(path: string, body: unknown): Promise<T> {
  await ensureCSRF();
  const response = await fetch(path, {
    method: 'PATCH',
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrfToken()
    },
    body: JSON.stringify(body)
  });
  return parseEnvelope<T>(response);
}

export async function apiDelete<T>(path: string): Promise<T> {
  await ensureCSRF();
  const response = await fetch(path, {
    method: 'DELETE',
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      'X-CSRF-Token': csrfToken()
    }
  });
  return parseEnvelope<T>(response);
}

async function ensureCSRF(): Promise<void> {
  if (csrfToken()) {
    return;
  }
  await apiGet<{ csrfReady: boolean }>('/api/security/csrf');
}

function csrfToken(): string {
  if (typeof document === 'undefined') {
    return '';
  }
  const cookie = document.cookie
    .split('; ')
    .find((item) => item.startsWith('omo_csrf='));
  return cookie ? decodeURIComponent(cookie.slice('omo_csrf='.length)) : '';
}

async function parseEnvelope<T>(response: Response): Promise<T> {
  const contentType = response.headers?.get?.('content-type') ?? 'application/json';
  if (!contentType.includes('application/json')) {
    throw new ApiError(
      response.ok
        ? '接口返回格式异常，请刷新页面或检查 OMO 服务状态。'
        : `接口请求失败（HTTP ${response.status}），请检查登录状态和服务日志。`,
      { code: 'NON_JSON_RESPONSE', status: response.status }
    );
  }
  const payload = (await response.json()) as Envelope<T>;
  if (!response.ok || !payload.success) {
    throw new ApiError(payload.error?.message ?? 'Request failed', {
      code: payload.error?.code,
      details: payload.error?.details,
      status: response.status
    });
  }
  return payload.data as T;
}
