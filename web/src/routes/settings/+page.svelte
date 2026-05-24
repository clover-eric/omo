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
    type BackupListResult,
    type BackupRecord,
    type BackupCreateResult,
    type BackupRestoreResult,
    type SettingsResponse,
    type UpdateCheckResult,
    type UpdateJobResult
  } from '$lib/api';

  let backups = $state<BackupRecord[]>([]);
  let updateCheck = $state<UpdateCheckResult | null>(null);
  let updateJob = $state<UpdateJobResult | null>(null);
  let loading = $state(true);
  let busy = $state('');
  let errorMessage = $state('');
  let successMessage = $state('');
  let updateManifestUrl = $state('');
  let providerEnabled = $state(false);
  let providerName = $state('Operator provider');
  let providerEndpoint = $state('');
  let providerTimeout = $state(3);
  let providerApiKey = $state('');
  let providerKeyConfigured = $state(false);
  let clearProviderKey = $state(false);
  let restoreConfirmId = $state('');
  let applyConfirmed = $state(false);
  let rollbackConfirmed = $state(false);

  onMount(() => {
    void loadSettingsPage();
  });

  async function loadSettingsPage() {
    loading = true;
    errorMessage = '';
    try {
      await Promise.all([loadSettings(), loadBackups(), checkUpdate()]);
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      loading = false;
    }
  }

  async function loadSettings() {
    const result = await apiGet<SettingsResponse>('/api/settings');
    const provider = result.diagnosticsExternalProvider;
    providerEnabled = provider.enabled;
    providerName = provider.name;
    providerEndpoint = provider.endpointUrl;
    providerTimeout = provider.timeoutSeconds;
    providerKeyConfigured = provider.apiKeyConfigured;
    updateManifestUrl = result.updateManifestUrl ?? '';
    providerApiKey = '';
    clearProviderKey = false;
  }

  async function loadBackups() {
    const result = await apiGet<BackupListResult>('/api/backups');
    backups = result.backups;
  }

  async function checkUpdate() {
    updateCheck = await apiGet<UpdateCheckResult>('/api/update/check');
  }

  async function saveSettings() {
    busy = 'settings';
    clearMessages();
    try {
      const result = await apiPatch<SettingsResponse>('/api/settings', {
        diagnosticsExternalProvider: {
          enabled: providerEnabled,
          name: providerName,
          endpointUrl: providerEndpoint,
          timeoutSeconds: Number(providerTimeout),
          apiKey: providerApiKey,
          clearApiKey: clearProviderKey
        },
        updateManifestUrl
      });
      const provider = result.diagnosticsExternalProvider;
      providerEnabled = provider.enabled;
      providerName = provider.name;
      providerEndpoint = provider.endpointUrl;
      providerTimeout = provider.timeoutSeconds;
      providerKeyConfigured = provider.apiKeyConfigured;
      updateManifestUrl = result.updateManifestUrl ?? '';
      providerApiKey = '';
      clearProviderKey = false;
      successMessage = 'Settings saved.';
      await checkUpdate();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busy = '';
    }
  }

  async function createBackup() {
    busy = 'backup';
    clearMessages();
    try {
      const result = await apiPost<BackupCreateResult>('/api/backups', {});
      successMessage = result.job.userMessage;
      await loadBackups();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busy = '';
    }
  }

  async function restoreBackup(backup: BackupRecord) {
    busy = backup.id;
    clearMessages();
    try {
      const result = await apiPost<BackupRestoreResult>(`/api/backups/${backup.id}/restore`, {
        confirm: true
      });
      successMessage = result.job.userMessage;
      restoreConfirmId = '';
      await loadBackups();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busy = '';
    }
  }

  async function refreshUpdate() {
    busy = 'update-check';
    clearMessages();
    try {
      await checkUpdate();
      successMessage = 'Update status refreshed.';
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busy = '';
    }
  }

  async function applyUpdate() {
    busy = 'update-apply';
    clearMessages();
    try {
      updateJob = await apiPost<UpdateJobResult>('/api/update/apply', { confirm: applyConfirmed });
      successMessage = updateJob.job.userMessage;
      applyConfirmed = false;
      await checkUpdate();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busy = '';
    }
  }

  async function rollbackUpdate() {
    busy = 'update-rollback';
    clearMessages();
    try {
      updateJob = await apiPost<UpdateJobResult>('/api/update/rollback', { confirm: rollbackConfirmed });
      successMessage = updateJob.job.userMessage;
      rollbackConfirmed = false;
      await checkUpdate();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busy = '';
    }
  }

  function clearMessages() {
    errorMessage = '';
    successMessage = '';
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : 'Settings operation failed.';
  }
</script>

<svelte:head>
  <title>Settings - OMO Boundary Operations</title>
  <meta
    name="description"
    content="Manage OMO backup, update, and diagnostics provider settings."
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
      <a href="/diagnostics">
        <Activity size={18} strokeWidth={1.8} />
        <span>Server Checkup</span>
      </a>
      <a href="/logs">
        <ClipboardList size={18} strokeWidth={1.8} />
        <span>Audit Logs</span>
      </a>
      <a class="active" href="/settings">
        <SlidersHorizontal size={18} strokeWidth={1.8} />
        <span>Settings</span>
      </a>
    </nav>
  </aside>

  <main class="workspace">
    <header class="topbar">
      <div>
        <p class="eyebrow">Operations</p>
        <h1>Settings</h1>
      </div>
      <button class="icon-button" type="button" aria-label="Refresh settings" onclick={loadSettingsPage} disabled={loading}>
        <RefreshCw size={17} class={loading ? 'spin' : ''} />
      </button>
    </header>

    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}
    {#if successMessage}
      <p class="success-text">{successMessage}</p>
    {/if}

    <section class="summary-grid" aria-label="Settings summary">
      <article class="metric-card">
        <div class="metric-icon"><RefreshCw size={20} /></div>
        <div>
          <p>Current version</p>
          <strong>{updateCheck?.currentVersion ?? '--'}</strong>
          <span>{updateCheck?.platform ?? 'Update status not loaded.'}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><CheckCircle2 size={20} /></div>
        <div>
          <p>Update</p>
          <strong>{updateCheck?.updateAvailable ? 'Available' : 'Current'}</strong>
          <span>{updateCheck?.summary ?? 'No update check result.'}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><ClipboardList size={20} /></div>
        <div>
          <p>Backups</p>
          <strong>{backups.length}</strong>
          <span>{backups.filter((item) => item.status === 'ready').length} ready archive records.</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><TriangleAlert size={20} /></div>
        <div>
          <p>Provider</p>
          <strong>{providerEnabled ? 'Enabled' : 'Disabled'}</strong>
          <span>{providerName}</span>
        </div>
      </article>
    </section>

    <section class="settings-grid">
      <form class="panel provider-form" onsubmit={(event) => { event.preventDefault(); saveSettings(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Release Channel</p>
            <h2>Update Manifest</h2>
          </div>
          <RefreshCw size={20} />
        </div>

        <label>
          <span>HTTPS manifest URL</span>
          <input bind:value={updateManifestUrl} placeholder="https://updates.example/manifest.json" />
        </label>

        <div class="row-actions">
          <button class="primary-action" type="submit" disabled={busy !== ''}>
            {#if busy === 'settings'}
              <LoaderCircle size={17} class="spin" />
            {:else}
              <SlidersHorizontal size={17} />
            {/if}
            Save Settings
          </button>
          <button class="primary-action secondary-action" type="button" onclick={refreshUpdate} disabled={busy !== ''}>
            <RefreshCw size={17} class={busy === 'update-check' ? 'spin' : ''} />
            Check Update
          </button>
        </div>

        {#if updateCheck}
          <dl class="snapshot-list">
            <div><dt>Latest</dt><dd>{updateCheck.latestVersion ?? 'Not configured'}</dd></div>
            <div><dt>Channel</dt><dd>{updateCheck.channel ?? 'Default'}</dd></div>
            <div><dt>Artifact</dt><dd>{updateCheck.artifactUrl ?? 'No matching artifact'}</dd></div>
            <div><dt>Checksum</dt><dd>{updateCheck.checksumSha256 ?? 'Not reported'}</dd></div>
          </dl>
        {/if}

        <label class="toggle-row">
          <input bind:checked={applyConfirmed} type="checkbox" />
          <span>Confirm update apply</span>
        </label>
        <label class="toggle-row">
          <input bind:checked={rollbackConfirmed} type="checkbox" />
          <span>Confirm update rollback</span>
        </label>

        <div class="row-actions">
          <button class="primary-action" type="button" onclick={applyUpdate} disabled={busy !== '' || !applyConfirmed}>
            <CheckCircle2 size={17} />
            Apply Update
          </button>
          <button class="primary-action danger-action" type="button" onclick={rollbackUpdate} disabled={busy !== '' || !rollbackConfirmed}>
            <RefreshCw size={17} />
            Rollback
          </button>
        </div>

        {#if updateJob}
          <p class="empty-text">{updateJob.job.userMessage}</p>
        {/if}
      </form>

      <form class="panel provider-form" onsubmit={(event) => { event.preventDefault(); saveSettings(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Diagnostics</p>
            <h2>Optional Provider</h2>
          </div>
          <Activity size={20} />
        </div>

        <label class="toggle-row">
          <input bind:checked={providerEnabled} type="checkbox" />
          <span>Enable during server checkup</span>
        </label>
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
        <label class="toggle-row">
          <input bind:checked={clearProviderKey} type="checkbox" />
          <span>Clear saved credential</span>
        </label>
        <button class="primary-action" type="submit" disabled={busy !== ''}>
          <SlidersHorizontal size={17} />
          Save Provider
        </button>
      </form>
    </section>

    <section class="panel service-section">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Backup And Restore</p>
          <h2>Encrypted Backup Records</h2>
        </div>
        <button class="primary-action" type="button" onclick={createBackup} disabled={busy !== ''}>
          {#if busy === 'backup'}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <ClipboardList size={17} />
          {/if}
          Create Backup
        </button>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>Loading backup records...</span>
        </div>
      {:else if backups.length === 0}
        <p class="empty-text">No backup records have been created yet.</p>
      {:else}
        <div class="subscription-list">
          {#each backups as backup}
            <article class="subscription-row">
              <div>
                <h3>{backup.status} backup</h3>
                <p>{new Date(backup.createdAt).toLocaleString()} - {backup.path}</p>
                {#if backup.checksum}
                  <code>{backup.checksum}</code>
                {/if}
              </div>
              <div class="row-actions">
                <label class="toggle-row">
                  <input
                    checked={restoreConfirmId === backup.id}
                    type="checkbox"
                    onchange={(event) => {
                      restoreConfirmId = (event.currentTarget as HTMLInputElement).checked ? backup.id : '';
                    }}
                  />
                  <span>Confirm</span>
                </label>
                <button
                  class="primary-action danger-action"
                  type="button"
                  onclick={() => restoreBackup(backup)}
                  disabled={busy !== '' || restoreConfirmId !== backup.id || backup.status !== 'ready'}
                >
                  <RefreshCw size={16} />
                  Restore
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>
  </main>
</div>
