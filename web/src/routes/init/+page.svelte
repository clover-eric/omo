<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import KeyRound from '@lucide/svelte/icons/key-round';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import { onMount } from 'svelte';
  import { apiGet, apiPost, type BootstrapEvent, type BootstrapJob, type BootstrapStatus } from '$lib/api';
  import { formatBootstrapState, type BootstrapState } from '$lib/status';

  let token = $state('');
  let username = $state('admin');
  let password = $state('');
  let confirmPassword = $state('');
  let domain = $state('');
  let loading = $state(true);
  let submitting = $state(false);
  let status = $state<BootstrapStatus | null>(null);
  let latestJob = $state<BootstrapJob | null>(null);
  let events = $state<BootstrapEvent[]>([]);
  let errorMessage = $state('');

  const steps = [
    'PREFLIGHT_CHECK',
    'ADMIN_CREATE',
    'DOMAIN_VERIFY',
    'TLS_PROVISION',
    'PANEL_HTTPS_ENABLE',
    'CORE_INSTALL',
    'FINAL_HEALTH_CHECK'
  ] as BootstrapState[];

  onMount(async () => {
    token = new URLSearchParams(window.location.search).get('token') ?? '';
    await loadStatus();
    loading = false;
    connectEvents();
  });

  async function loadStatus() {
    try {
      status = await apiGet<BootstrapStatus>('/api/bootstrap/status');
      latestJob = status.latestJob ?? null;
      if (status.domain) {
        domain = status.domain;
      }
    } catch (error) {
      errorMessage = messageFrom(error);
    }
  }

  async function startBootstrap(retry = false) {
    errorMessage = '';
    submitting = true;
    try {
      const result = await apiPost<{ job: BootstrapJob; redirectTo: string }>('/api/bootstrap/start', {
        token,
        username,
        password,
        confirmPassword,
        domain,
        retry
      });
      latestJob = result.job;
      await loadStatus();
      if (result.redirectTo) {
        window.setTimeout(() => {
          window.location.href = result.redirectTo;
        }, 3000);
      }
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      submitting = false;
    }
  }

  function connectEvents() {
    const source = new EventSource('/api/bootstrap/events');
    source.addEventListener('bootstrap', (event) => {
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
    source.addEventListener('error', () => {
      if (events.length === 0) {
        errorMessage = 'Initialization events are unavailable. Refresh and retry.';
      }
    });
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : 'Request failed. Please retry.';
  }

  const currentProgress = $derived(latestJob?.progress ?? 0);
  const complete = $derived(status?.phase1Complete || latestJob?.status === 'succeeded');
</script>

<svelte:head>
  <title>Initialization - OMO Boundary Operations</title>
</svelte:head>

<main class="init-page">
  <section class="init-hero">
    <div class="init-heading">
      <div class="init-mark">
        <ShieldCheck size={24} />
      </div>
      <div>
        <p>OMO Boundary Operations Platform</p>
        <h1>Initialization</h1>
      </div>
    </div>
    <div class="progress-ring" aria-label="Initialization progress">
      <span>{currentProgress}%</span>
    </div>
  </section>

  <section class="init-grid">
    <form class="init-form" onsubmit={(event) => { event.preventDefault(); startBootstrap(false); }}>
      <div class="form-title">
        <KeyRound size={20} />
        <h2>Administrator And Domain</h2>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>Loading initialization status...</span>
        </div>
      {:else}
        <label>
          <span>One-time initialization token</span>
          <input bind:value={token} autocomplete="one-time-code" required />
        </label>

        <label>
          <span>Administrator username</span>
          <input bind:value={username} autocomplete="username" required minlength="3" maxlength="64" />
        </label>

        <label>
          <span>Administrator password</span>
          <input bind:value={password} type="password" autocomplete="new-password" required minlength="12" />
        </label>

        <label>
          <span>Confirm password</span>
          <input bind:value={confirmPassword} type="password" autocomplete="new-password" required minlength="12" />
        </label>

        <label>
          <span>Domain resolving to this server</span>
          <input bind:value={domain} inputmode="url" placeholder="ops.example.com" required />
        </label>

        {#if errorMessage}
          <p class="error-text">{errorMessage}</p>
        {/if}

        <button type="submit" disabled={submitting || complete}>
          {#if submitting}
            <LoaderCircle size={18} class="spin" />
          {:else if complete}
            <CheckCircle2 size={18} />
          {:else}
            <Activity size={18} />
          {/if}
          <span>{complete ? 'Entry configuration complete' : 'Start automated configuration'}</span>
        </button>

        {#if latestJob?.status === 'failed'}
          <button class="secondary-button" type="button" disabled={submitting} onclick={() => startBootstrap(true)}>
            <Activity size={18} />
            <span>Retry initialization</span>
          </button>
        {/if}
      {/if}
    </form>

    <aside class="init-status">
      <div class="status-header">
        <p>Live steps</p>
        <strong>{formatBootstrapState((latestJob?.state ?? status?.state ?? 'UNINITIALIZED') as BootstrapState)}</strong>
      </div>

      <div class="bar">
        <span style={`width: ${currentProgress}%`}></span>
      </div>

      <div class="step-list">
        {#each steps as step}
          <div class:done={currentProgress >= ((steps.indexOf(step) + 1) / steps.length) * 100}>
            <span></span>
            <p>{formatBootstrapState(step)}</p>
          </div>
        {/each}
      </div>

      <div class="event-log" aria-label="Initialization log">
        {#if events.length === 0}
          <p class="muted">Waiting for initialization events</p>
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
</main>
