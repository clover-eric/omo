<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import Network from '@lucide/svelte/icons/network';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import { onMount } from 'svelte';
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
      settingsMessage = 'Provider settings saved.';
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
    return error instanceof Error ? error.message : 'Server checkup failed.';
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
  <title>Server Checkup - OMO Boundary Operations</title>
  <meta
    name="description"
    content="Run authorized OMO server checkups for local service health and resource evidence."
  />
</svelte:head>

<div class="shell">
  <aside class="sidebar" aria-label="Primary navigation">
    <div class="brand">
      <div class="brand-mark">O</div>
      <div>
        <strong>OMO</strong>
        <span>Boundary Operations</span>
      </div>
    </div>

    <nav class="nav-list">
      <a href="/services">
        <ShieldCheck size={18} strokeWidth={1.8} />
        <span>Service Library</span>
      </a>
      <a href="/subscriptions">
        <ClipboardList size={18} strokeWidth={1.8} />
        <span>Distribution</span>
      </a>
      <a href="/cascade">
        <Network size={18} strokeWidth={1.8} />
        <span>Cascade Nodes</span>
      </a>
      <a class="active" href="/diagnostics">
        <Activity size={18} strokeWidth={1.8} />
        <span>Server Checkup</span>
      </a>
      <a href="/logs">
        <ClipboardList size={18} strokeWidth={1.8} />
        <span>Audit Logs</span>
      </a>
      <a href="/settings">
        <SlidersHorizontal size={18} strokeWidth={1.8} />
        <span>Settings</span>
      </a>
    </nav>
  </aside>

  <main class="workspace">
    <header class="topbar">
      <div>
        <p class="eyebrow">Phase 5</p>
        <h1>Server Checkup</h1>
      </div>
      <div class="toolbar-actions">
        <button class="icon-button" type="button" aria-label="Refresh report" onclick={loadLatest} disabled={loading}>
          <RefreshCw size={17} class={loading ? 'spin' : ''} />
        </button>
        <button class="primary-action" type="button" onclick={runDiagnostics} disabled={running}>
          {#if running}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <Activity size={17} />
          {/if}
          Run Checkup
        </button>
      </div>
    </header>

    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}

    <section class="summary-grid" aria-label="Server checkup summary">
      <article class="metric-card">
        <div class="metric-icon">
          <Activity size={20} />
        </div>
        <div>
          <p>Status</p>
          <strong>{report?.status ?? 'No report'}</strong>
          <span>{report?.summary ?? 'Run a server checkup to collect local evidence.'}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon">
          <ShieldCheck size={20} />
        </div>
        <div>
          <p>Job</p>
          <strong>{latestJob?.status ?? 'Idle'}</strong>
          <span>{latestJob?.userMessage ?? 'No active checkup job.'}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon">
          <ClipboardList size={20} />
        </div>
        <div>
          <p>Checks</p>
          <strong>{report?.checks.length ?? 0}</strong>
          <span>Local evidence plus explicitly enabled providers.</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon">
          <CheckCircle2 size={20} />
        </div>
        <div>
          <p>Updated</p>
          <strong>{report ? new Date(report.createdAt).toLocaleTimeString() : '--'}</strong>
          <span>{report ? new Date(report.createdAt).toLocaleDateString() : 'No saved report.'}</span>
        </div>
      </article>
    </section>

    <section class="diagnostics-grid">
      <section class="panel">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Evidence</p>
            <h2>Check Results</h2>
          </div>
        </div>

        {#if loading}
          <div class="loading-row">
            <LoaderCircle size={18} class="spin" />
            <span>Loading server checkup report...</span>
          </div>
        {:else if !report}
          <p class="empty-text">No server checkup report has been created yet.</p>
        {:else}
          <div class="check-result-list">
            {#each report.checks as check}
              <article class:warning={check.status === 'warning'} class:error={check.status === 'error'}>
                <div class="check-icon">
                  <svelte:component this={checkIcon(check)} size={18} />
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
            <p class="eyebrow">Runtime</p>
            <h2>System Snapshot</h2>
          </div>
        </div>

        {#if report}
          <dl class="snapshot-list">
            <div><dt>Host</dt><dd>{report.system.hostname || 'unknown'}</dd></div>
            <div><dt>OS</dt><dd>{report.system.os}/{report.system.architecture}</dd></div>
            <div><dt>CPU</dt><dd>{report.system.cpuCount} logical</dd></div>
            <div><dt>Go</dt><dd>{report.system.goVersion}</dd></div>
            <div><dt>Memory</dt><dd>{report.system.memoryAllocMb} MB allocated</dd></div>
          </dl>
        {:else}
          <p class="empty-text">System snapshot will appear after the first checkup.</p>
        {/if}

        <div class="event-log diagnostics-events" aria-label="Server checkup events">
          {#if events.length === 0}
            <p class="muted">Waiting for server checkup events</p>
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

    <section class="panel provider-settings" aria-label="Optional provider settings">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Optional Provider</p>
          <h2>Operator Configured Check</h2>
        </div>
        <SlidersHorizontal size={20} />
      </div>

      {#if settingsMessage}
        <p class={settingsMessage === 'Provider settings saved.' ? 'success-text' : 'error-text'}>{settingsMessage}</p>
      {/if}

      <form class="provider-form" onsubmit={(event) => { event.preventDefault(); saveProviderSettings(); }}>
        <label class="toggle-row">
          <input bind:checked={providerEnabled} type="checkbox" />
          <span>Enable during server checkup</span>
        </label>

        <div class="provider-fields">
          <label>
            <span>Name</span>
            <input bind:value={providerName} maxlength="80" />
          </label>

          <label>
            <span>HTTPS endpoint</span>
            <input bind:value={providerEndpoint} placeholder="https://provider.example/check" />
          </label>

          <label>
            <span>Timeout</span>
            <input bind:value={providerTimeout} min="1" max="10" type="number" />
          </label>

          <label>
            <span>API key</span>
            <input bind:value={providerApiKey} placeholder={providerKeyConfigured ? 'Saved credential is configured' : 'Optional credential'} type="password" />
          </label>
        </div>

        <label class="toggle-row">
          <input bind:checked={clearProviderKey} type="checkbox" />
          <span>Clear saved credential</span>
        </label>

        <button class="primary-action" type="submit" disabled={savingProvider}>
          {#if savingProvider}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <SlidersHorizontal size={17} />
          {/if}
          Save Provider
        </button>
      </form>
    </section>
  </main>
</div>
