<script lang="ts">
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { preferences, type Language } from '$lib/preferences';
  import { apiGet, type AuditListResult, type AuditLog } from '$lib/api';

  let logs = $state<AuditLog[]>([]);
  let loading = $state(true);
  let errorMessage = $state('');
  let limit = $state(100);
  let t = $derived(
    $preferences.language === 'zh-CN'
      ? {
          title: '审计日志',
          eyebrow: '审计',
          refresh: '刷新审计日志',
          limit: '数量',
          records: '记录',
          recordsNote: '后端返回的最新审计记录。',
          latestAction: '最近操作',
          none: '无',
          noEntries: '暂无保存的审计记录。',
          recent: '近期活动',
          operations: '管理员操作',
          summaryLabel: '审计概览',
          loading: '正在加载审计记录...',
          empty: '尚无审计记录。',
          failed: '无法加载审计日志。'
        }
      : {
          title: 'Audit Logs',
          eyebrow: 'Audit',
          refresh: 'Refresh audit logs',
          limit: 'Limit',
          records: 'Records',
          recordsNote: 'Newest audit entries returned by the backend.',
          latestAction: 'Latest action',
          none: 'None',
          noEntries: 'No saved audit entries.',
          recent: 'Recent Activity',
          operations: 'Administrator Operations',
          summaryLabel: 'Audit summary',
          loading: 'Loading audit records...',
          empty: 'No audit records are available yet.',
          failed: 'Audit logs could not be loaded.'
        }
  );

  onMount(() => {
    void loadLogs();
  });

  async function loadLogs() {
    loading = true;
    errorMessage = '';
    try {
      const result = await apiGet<AuditListResult>(`/api/audit?limit=${Number(limit)}`);
      logs = result.logs ?? [];
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      loading = false;
    }
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : t.failed;
  }

  function detailsText(details: Record<string, unknown>) {
    return JSON.stringify(details ?? {}, null, 2);
  }
</script>

<svelte:head>
  <title>{t.title} - OMO</title>
  <meta
    name="description"
    content="Review OMO administrator and infrastructure operation audit records."
  />
</svelte:head>

{#snippet actions()}
  <label class="compact-field">
    <span>{t.limit}</span>
    <input bind:value={limit} min="1" max="200" type="number" />
  </label>
  <button class="icon-button" type="button" aria-label={t.refresh} onclick={loadLogs} disabled={loading}>
    <RefreshCw size={17} class={loading ? 'spin' : ''} />
  </button>
{/snippet}

<ConsoleShell title={t.title} eyebrow={t.eyebrow} activeHref="/logs" {actions}>
    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}

    <section class="summary-grid" aria-label={t.summaryLabel}>
      <article class="metric-card">
        <div class="metric-icon"><ClipboardList size={20} /></div>
        <div>
          <p>{t.records}</p>
          <strong>{logs.length}</strong>
          <span>{t.recordsNote}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><ShieldCheck size={20} /></div>
        <div>
          <p>{t.latestAction}</p>
          <strong>{logs[0]?.action ?? t.none}</strong>
          <span>{logs[0] ? new Date(logs[0].createdAt).toLocaleString() : t.noEntries}</span>
        </div>
      </article>
    </section>

    <section class="panel">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.recent}</p>
          <h2>{t.operations}</h2>
        </div>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>{t.loading}</span>
        </div>
      {:else if logs.length === 0}
        <p class="empty-text">{t.empty}</p>
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
</ConsoleShell>
