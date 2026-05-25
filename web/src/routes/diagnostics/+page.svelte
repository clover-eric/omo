<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { preferences, type Language } from '$lib/preferences';
  import {
    apiGet,
    apiPatch,
    apiPost,
    type BootstrapEvent,
    type BootstrapJob,
    type DiagnosticCheck,
    type DiagnosticReport,
    type DiagnosticsLatestResult,
    type DiagnosticsRunResult,
    type SettingsResponse
  } from '$lib/api';

  type Copy = {
    title: string;
    phase: string;
    refresh: string;
    run: string;
    status: string;
    noReport: string;
    runToCollect: string;
    job: string;
    idle: string;
    noActiveJob: string;
    checks: string;
    checksNote: string;
    updated: string;
    noSavedReport: string;
    evidence: string;
    checkResults: string;
    loading: string;
    empty: string;
    runtime: string;
    snapshot: string;
    host: string;
    os: string;
    cpu: string;
    memory: string;
    snapshotEmpty: string;
    waitingEvents: string;
    provider: string;
    providerTitle: string;
    settingsSaved: string;
    enableProvider: string;
    name: string;
    endpoint: string;
    timeout: string;
    apiKey: string;
    savedCredential: string;
    optionalCredential: string;
    clearCredential: string;
    saveProvider: string;
    failed: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '服务器体检',
      phase: '第五阶段',
      refresh: '刷新体检报告',
      run: '运行体检',
      status: '状态',
      noReport: '暂无报告',
      runToCollect: '运行服务器体检以采集本机证据。',
      job: '任务',
      idle: '空闲',
      noActiveJob: '当前没有正在运行的体检任务。',
      checks: '检查项',
      checksNote: '本机证据和显式启用的提供方。',
      updated: '更新时间',
      noSavedReport: '暂无保存的报告。',
      evidence: '证据',
      checkResults: '检查结果',
      loading: '正在加载服务器体检报告...',
      empty: '尚未创建服务器体检报告。',
      runtime: '运行时',
      snapshot: '系统快照',
      host: '主机',
      os: '系统',
      cpu: 'CPU',
      memory: '内存',
      snapshotEmpty: '首次体检后会显示系统快照。',
      waitingEvents: '等待服务器体检事件',
      provider: '可选提供方',
      providerTitle: '运维方配置检查',
      settingsSaved: '提供方设置已保存。',
      enableProvider: '体检时启用',
      name: '名称',
      endpoint: 'HTTPS 端点',
      timeout: '超时',
      apiKey: 'API 密钥',
      savedCredential: '已保存凭据',
      optionalCredential: '可选凭据',
      clearCredential: '清除已保存凭据',
      saveProvider: '保存提供方',
      failed: '服务器体检失败。'
    },
    'en-US': {
      title: 'Server Checkup',
      phase: 'Phase 5',
      refresh: 'Refresh report',
      run: 'Run Checkup',
      status: 'Status',
      noReport: 'No report',
      runToCollect: 'Run a server checkup to collect local evidence.',
      job: 'Job',
      idle: 'Idle',
      noActiveJob: 'No active checkup job.',
      checks: 'Checks',
      checksNote: 'Local evidence plus explicitly enabled providers.',
      updated: 'Updated',
      noSavedReport: 'No saved report.',
      evidence: 'Evidence',
      checkResults: 'Check Results',
      loading: 'Loading server checkup report...',
      empty: 'No server checkup report has been created yet.',
      runtime: 'Runtime',
      snapshot: 'System Snapshot',
      host: 'Host',
      os: 'OS',
      cpu: 'CPU',
      memory: 'Memory',
      snapshotEmpty: 'System snapshot will appear after the first checkup.',
      waitingEvents: 'Waiting for server checkup events',
      provider: 'Optional Provider',
      providerTitle: 'Operator Configured Check',
      settingsSaved: 'Provider settings saved.',
      enableProvider: 'Enable during server checkup',
      name: 'Name',
      endpoint: 'HTTPS endpoint',
      timeout: 'Timeout',
      apiKey: 'API key',
      savedCredential: 'Saved credential is configured',
      optionalCredential: 'Optional credential',
      clearCredential: 'Clear saved credential',
      saveProvider: 'Save Provider',
      failed: 'Server checkup failed.'
    }
  };

  let report = $state<DiagnosticReport | null>(null);
  let latestJob = $state<BootstrapJob | null>(null);
  let events = $state<BootstrapEvent[]>([]);
  let loading = $state(true);
  let running = $state(false);
  let savingProvider = $state(false);
  let errorMessage = $state('');
  let settingsMessage = $state('');
  let providerEnabled = $state(false);
  let providerName = $state('Operator provider');
  let providerEndpoint = $state('');
  let providerTimeout = $state(3);
  let providerApiKey = $state('');
  let providerKeyConfigured = $state(false);
  let clearProviderKey = $state(false);
  let t = $derived(copy[$preferences.language]);

  onMount(() => {
    void loadLatest();
    void loadSettings();
    connectEvents();
  });

  async function loadLatest() {
    loading = true;
    errorMessage = '';
    try {
      const result = await apiGet<DiagnosticsLatestResult>('/api/diagnostics/latest');
      report = result.report;
      latestJob = result.latestJob ?? null;
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      loading = false;
    }
  }

  async function loadSettings() {
    settingsMessage = '';
    try {
      const result = await apiGet<SettingsResponse>('/api/settings');
      const provider = result.diagnosticsExternalProvider;
      providerEnabled = provider.enabled;
      providerName = provider.name;
      providerEndpoint = provider.endpointUrl;
      providerTimeout = provider.timeoutSeconds;
      providerKeyConfigured = provider.apiKeyConfigured;
      providerApiKey = '';
      clearProviderKey = false;
    } catch (error) {
      settingsMessage = messageFrom(error);
    }
  }

  async function saveProviderSettings() {
    savingProvider = true;
    settingsMessage = '';
    try {
      const result = await apiPatch<SettingsResponse>('/api/settings', {
        diagnosticsExternalProvider: {
          enabled: providerEnabled,
          name: providerName,
          endpointUrl: providerEndpoint,
          timeoutSeconds: Number(providerTimeout),
          apiKey: providerApiKey,
          clearApiKey: clearProviderKey
        }
      });
      const provider = result.diagnosticsExternalProvider;
      providerEnabled = provider.enabled;
      providerName = provider.name;
      providerEndpoint = provider.endpointUrl;
      providerTimeout = provider.timeoutSeconds;
      providerKeyConfigured = provider.apiKeyConfigured;
      providerApiKey = '';
      clearProviderKey = false;
      settingsMessage = t.settingsSaved;
    } catch (error) {
      settingsMessage = messageFrom(error);
    } finally {
      savingProvider = false;
    }
  }

  async function runDiagnostics() {
    running = true;
    errorMessage = '';
    try {
      const result = await apiPost<DiagnosticsRunResult>('/api/diagnostics/run', {});
      report = result.report;
      latestJob = result.job;
      await loadLatest();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      running = false;
    }
  }

  function connectEvents() {
    const source = new EventSource('/api/diagnostics/events');
    source.addEventListener('diagnostics', (event) => {
      const item = JSON.parse((event as MessageEvent).data) as BootstrapEvent;
      events = [...events, item].slice(-80);
      latestJob = {
        id: item.jobId,
        kind: item.kind,
        state: item.state,
        status: item.status,
        progress: item.progress,
        userMessage: item.message
      };
    });
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : t.failed;
  }

  function checkIcon(check: DiagnosticCheck) {
    if (check.status === 'ok') {
      return CheckCircle2;
    }
    if (check.status === 'warning') {
      return TriangleAlert;
    }
    return Activity;
  }
</script>

<svelte:head>
  <title>{t.title} - OMO</title>
  <meta
    name="description"
    content="Run authorized OMO server checkups for local service health and resource evidence."
  />
</svelte:head>

{#snippet actions()}
  <button class="icon-button" type="button" aria-label={t.refresh} onclick={loadLatest} disabled={loading}>
    <RefreshCw size={17} class={loading ? 'spin' : ''} />
  </button>
  <button class="primary-action" type="button" onclick={runDiagnostics} disabled={running}>
    {#if running}
      <LoaderCircle size={17} class="spin" />
    {:else}
      <Activity size={17} />
    {/if}
    {t.run}
  </button>
{/snippet}

<ConsoleShell title={t.title} eyebrow={t.phase} activeHref="/diagnostics" {actions}>
  {#if errorMessage}
    <p class="error-text">{errorMessage}</p>
  {/if}

  <section class="summary-grid" aria-label={t.title}>
    <article class="metric-card">
      <div class="metric-icon"><Activity size={20} /></div>
      <div>
        <p>{t.status}</p>
        <strong>{report?.status ?? t.noReport}</strong>
        <span>{report?.summary ?? t.runToCollect}</span>
      </div>
    </article>
    <article class="metric-card">
      <div class="metric-icon"><ShieldCheck size={20} /></div>
      <div>
        <p>{t.job}</p>
        <strong>{latestJob?.status ?? t.idle}</strong>
        <span>{latestJob?.userMessage ?? t.noActiveJob}</span>
      </div>
    </article>
    <article class="metric-card">
      <div class="metric-icon"><ClipboardList size={20} /></div>
      <div>
        <p>{t.checks}</p>
        <strong>{report?.checks.length ?? 0}</strong>
        <span>{t.checksNote}</span>
      </div>
    </article>
    <article class="metric-card">
      <div class="metric-icon"><CheckCircle2 size={20} /></div>
      <div>
        <p>{t.updated}</p>
        <strong>{report ? new Date(report.createdAt).toLocaleTimeString() : '--'}</strong>
        <span>{report ? new Date(report.createdAt).toLocaleDateString() : t.noSavedReport}</span>
      </div>
    </article>
  </section>

  <section class="diagnostics-grid">
    <section class="panel">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.evidence}</p>
          <h2>{t.checkResults}</h2>
        </div>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>{t.loading}</span>
        </div>
      {:else if !report}
        <p class="empty-text">{t.empty}</p>
      {:else}
        <div class="check-result-list">
          {#each report.checks as check}
            {@const Icon = checkIcon(check)}
            <article class:warning={check.status === 'warning'} class:error={check.status === 'error'}>
              <div class="check-icon">
                <Icon size={18} />
              </div>
              <div>
                <h3>{check.label}</h3>
                <p>{check.message}</p>
                {#if check.evidence}
                  <code>{check.evidence}</code>
                {/if}
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>

    <aside class="panel">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.runtime}</p>
          <h2>{t.snapshot}</h2>
        </div>
      </div>

      {#if report}
        <dl class="snapshot-list">
          <div><dt>{t.host}</dt><dd>{report.system.hostname || 'unknown'}</dd></div>
          <div><dt>{t.os}</dt><dd>{report.system.os}/{report.system.architecture}</dd></div>
          <div><dt>{t.cpu}</dt><dd>{report.system.cpuCount} logical</dd></div>
          <div><dt>Go</dt><dd>{report.system.goVersion}</dd></div>
          <div><dt>{t.memory}</dt><dd>{report.system.memoryAllocMb} MB allocated</dd></div>
        </dl>
      {:else}
        <p class="empty-text">{t.snapshotEmpty}</p>
      {/if}

      <div class="event-log diagnostics-events" aria-label={t.title}>
        {#if events.length === 0}
          <p class="muted">{t.waitingEvents}</p>
        {:else}
          {#each events as event}
            <article>
              <span>{event.progress}%</span>
              <p>{event.message}</p>
            </article>
          {/each}
        {/if}
      </div>
    </aside>
  </section>

  <section class="panel provider-settings" aria-label={t.provider}>
    <div class="panel-heading">
      <div>
        <p class="eyebrow">{t.provider}</p>
        <h2>{t.providerTitle}</h2>
      </div>
      <SlidersHorizontal size={20} />
    </div>

    {#if settingsMessage}
      <p class={settingsMessage === t.settingsSaved ? 'success-text' : 'error-text'}>{settingsMessage}</p>
    {/if}

    <form class="provider-form" onsubmit={(event) => { event.preventDefault(); saveProviderSettings(); }}>
      <label class="toggle-row">
        <input bind:checked={providerEnabled} type="checkbox" />
        <span>{t.enableProvider}</span>
      </label>

      <div class="provider-fields">
        <label><span>{t.name}</span><input bind:value={providerName} maxlength="80" /></label>
        <label><span>{t.endpoint}</span><input bind:value={providerEndpoint} placeholder="https://provider.example/check" /></label>
        <label><span>{t.timeout}</span><input bind:value={providerTimeout} min="1" max="10" type="number" /></label>
        <label>
          <span>{t.apiKey}</span>
          <input bind:value={providerApiKey} placeholder={providerKeyConfigured ? t.savedCredential : t.optionalCredential} type="password" />
        </label>
      </div>

      <label class="toggle-row">
        <input bind:checked={clearProviderKey} type="checkbox" />
        <span>{t.clearCredential}</span>
      </label>

      <button class="primary-action" type="submit" disabled={savingProvider}>
        {#if savingProvider}
          <LoaderCircle size={17} class="spin" />
        {:else}
          <SlidersHorizontal size={17} />
        {/if}
        {t.saveProvider}
      </button>
    </form>
  </section>
</ConsoleShell>
