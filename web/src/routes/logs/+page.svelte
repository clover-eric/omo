<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import Network from '@lucide/svelte/icons/network';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import { onMount } from 'svelte';
  import { apiGet, type AuditListResult, type AuditLog } from '$lib/api';

  let logs = $state<AuditLog[]>([]);
  let loading = $state(true);
  let errorMessage = $state('');
  let limit = $state(100);

  onMount(() => {
    void loadLogs();
  });

  async function loadLogs() {
    loading = true;
    errorMessage = '';
    try {
      const result = await apiGet<AuditListResult>(`/api/audit?limit=${Number(limit)}`);
      logs = result.logs;
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      loading = false;
    }
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : 'Audit logs could not be loaded.';
  }

  function detailsText(details: Record<string, unknown>) {
    return JSON.stringify(details ?? {}, null, 2);
  }
</script>

<svelte:head>
  <title>Audit Logs - OMO Boundary Operations</title>
  <meta
    name="description"
    content="Review OMO administrator and infrastructure operation audit records."
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
      <a class="active" href="/logs">
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
        <p class="eyebrow">Audit</p>
        <h1>Audit Logs</h1>
      </div>
      <div class="toolbar-actions">
        <label class="compact-field">
          <span>Limit</span>
          <input bind:value={limit} min="1" max="200" type="number" />
        </label>
        <button class="icon-button" type="button" aria-label="Refresh audit logs" onclick={loadLogs} disabled={loading}>
          <RefreshCw size={17} class={loading ? 'spin' : ''} />
        </button>
      </div>
    </header>

    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}

    <section class="summary-grid" aria-label="Audit summary">
      <article class="metric-card">
        <div class="metric-icon"><ClipboardList size={20} /></div>
        <div>
          <p>Records</p>
          <strong>{logs.length}</strong>
          <span>Newest audit entries returned by the backend.</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><ShieldCheck size={20} /></div>
        <div>
          <p>Latest action</p>
          <strong>{logs[0]?.action ?? 'None'}</strong>
          <span>{logs[0] ? new Date(logs[0].createdAt).toLocaleString() : 'No saved audit entries.'}</span>
        </div>
      </article>
    </section>

    <section class="panel">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Recent Activity</p>
          <h2>Administrator Operations</h2>
        </div>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>Loading audit records...</span>
        </div>
      {:else if logs.length === 0}
        <p class="empty-text">No audit records are available yet.</p>
      {:else}
        <div class="audit-list">
          {#each logs as log}
            <article class="audit-row">
              <div>
                <h3>{log.action}</h3>
                <p>{log.resourceType}{log.resourceId ? ` - ${log.resourceId}` : ''}</p>
                <span>{new Date(log.createdAt).toLocaleString()}</span>
              </div>
              <code>{detailsText(log.details)}</code>
            </article>
          {/each}
        </div>
      {/if}
    </section>
  </main>
</div>
