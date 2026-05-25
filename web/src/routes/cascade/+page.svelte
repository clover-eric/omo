<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import Link2 from '@lucide/svelte/icons/link-2';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import Network from '@lucide/svelte/icons/network';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { preferences, type Language } from '$lib/preferences';
  import {
    apiDelete,
    apiGet,
    apiPatch,
    apiPost,
    type CascadeNode,
    type CascadeNodeList,
    type CascadePair,
    type CascadeConfigApplyResult,
    type CascadeConfigPlanResult,
    type CascadeHealthSample,
    type CascadeHealthSampleResult,
    type PairingAcceptResult,
    type PairingCodeResult
  } from '$lib/api';

  type Copy = {
    title: string;
    phase: string;
    refresh: string;
    sample: string;
    failed: string;
    summaryLabel: string;
    trustedNodes: string;
    totalRecords: string;
    oneHopLinks: string;
    appliedRecords: string;
    online: string;
    latestSamples: string;
    exitNode: string;
    createPairingCode: string;
    nodeName: string;
    domain: string;
    ttlMinutes: string;
    createCode: string;
    oneTimePairingCode: string;
    copied: string;
    copyCode: string;
    entryNode: string;
    acceptPairing: string;
    exitDomain: string;
    pairingCode: string;
    configurationPlan: string;
    oneHopPreview: string;
    trustRecords: string;
    knownNodes: string;
    loadingNodes: string;
    emptyNodes: string;
    noLinkRecord: string;
    offline: string;
    plan: string;
    apply: string;
    trust: string;
    disable: string;
    delete: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '级联节点',
      phase: '第六阶段',
      refresh: '刷新级联节点',
      sample: '采样级联健康',
      failed: '级联操作失败。',
      summaryLabel: '级联概览',
      trustedNodes: '可信节点',
      totalRecords: '条级联记录',
      oneHopLinks: '一跳链路',
      appliedRecords: '条已应用配置记录',
      online: '在线',
      latestSamples: '条最新健康样本',
      exitNode: '出口节点',
      createPairingCode: '创建配对码',
      nodeName: '节点名称',
      domain: '域名',
      ttlMinutes: '有效分钟数',
      createCode: '创建配对码',
      oneTimePairingCode: '一次性配对码',
      copied: '已复制',
      copyCode: '复制配对码',
      entryNode: '入口节点',
      acceptPairing: '接受配对',
      exitDomain: '出口域名',
      pairingCode: '配对码',
      configurationPlan: '配置计划',
      oneHopPreview: '一跳级联预览',
      trustRecords: '信任记录',
      knownNodes: '已知级联节点',
      loadingNodes: '正在加载级联节点...',
      emptyNodes: '尚未配对级联节点。',
      noLinkRecord: '无链路记录',
      offline: '离线',
      plan: '规划',
      apply: '应用',
      trust: '信任',
      disable: '停用',
      delete: '删除'
    },
    'en-US': {
      title: 'Cascade Nodes',
      phase: 'Phase 6',
      refresh: 'Refresh cascade nodes',
      sample: 'Sample cascade health',
      failed: 'Cascade operation failed.',
      summaryLabel: 'Cascade summary',
      trustedNodes: 'Trusted nodes',
      totalRecords: 'total cascade records',
      oneHopLinks: 'One-hop links',
      appliedRecords: 'applied configuration records',
      online: 'Online',
      latestSamples: 'latest health samples',
      exitNode: 'Exit Node',
      createPairingCode: 'Create Pairing Code',
      nodeName: 'Node name',
      domain: 'Domain',
      ttlMinutes: 'TTL minutes',
      createCode: 'Create Code',
      oneTimePairingCode: 'One-time pairing code',
      copied: 'Copied',
      copyCode: 'Copy Code',
      entryNode: 'Entry Node',
      acceptPairing: 'Accept Pairing',
      exitDomain: 'Exit domain',
      pairingCode: 'Pairing code',
      configurationPlan: 'Configuration Plan',
      oneHopPreview: 'One-Hop Cascade Preview',
      trustRecords: 'Trust Records',
      knownNodes: 'Known Cascade Nodes',
      loadingNodes: 'Loading cascade nodes...',
      emptyNodes: 'No cascade nodes have been paired yet.',
      noLinkRecord: 'no link record',
      offline: 'offline',
      plan: 'Plan',
      apply: 'Apply',
      trust: 'Trust',
      disable: 'Disable',
      delete: 'Delete'
    }
  };

  let nodes = $state<CascadeNode[]>([]);
  let pairs = $state<CascadePair[]>([]);
  let nodeName = $state('出口级联节点');
  let domain = $state('');
  let ttlMinutes = $state(15);
  let exitDomain = $state('');
  let pairingCode = $state('');
  let latestCode = $state<PairingCodeResult | null>(null);
  let loading = $state(true);
  let creating = $state(false);
  let accepting = $state(false);
  let busyNode = $state('');
  let busyPair = $state('');
  let sampling = $state(false);
  let errorMessage = $state('');
  let copied = $state(false);
  let latestPlan = $state<CascadeConfigPlanResult | null>(null);
  let latestApply = $state<CascadeConfigApplyResult | null>(null);
  let latestSamples = $state<CascadeHealthSample[]>([]);
  let healthMessage = $state('');
  let t = $derived(copy[$preferences.language]);

  onMount(() => {
    void loadCascade();
  });

  async function loadCascade() {
    loading = true;
    errorMessage = '';
    try {
      const result = await apiGet<CascadeNodeList>('/api/cascade/nodes');
      nodes = result.nodes ?? [];
      pairs = result.pairs ?? [];
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      loading = false;
    }
  }

  async function createCode() {
    creating = true;
    errorMessage = '';
    try {
      latestCode = await apiPost<PairingCodeResult>('/api/pairing/code', {
        nodeName,
        domain,
        ttlMinutes: Number(ttlMinutes)
      });
      pairingCode = latestCode.code;
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      creating = false;
    }
  }

  async function acceptCode() {
    accepting = true;
    errorMessage = '';
    try {
      await apiPost<PairingAcceptResult>('/api/pairing/accept', {
        exitDomain,
        code: pairingCode
      });
      pairingCode = '';
      await loadCascade();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      accepting = false;
    }
  }

  async function disableNode(node: CascadeNode) {
    busyNode = node.id;
    errorMessage = '';
    try {
      await apiPatch(`/api/cascade/nodes/${node.id}`, {
        name: node.name,
        status: node.status === 'disabled' ? 'trusted' : 'disabled'
      });
      await loadCascade();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busyNode = '';
    }
  }

  async function deleteNode(node: CascadeNode) {
    busyNode = node.id;
    errorMessage = '';
    try {
      await apiDelete(`/api/cascade/nodes/${node.id}`);
      await loadCascade();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busyNode = '';
    }
  }

  async function planConfig(pair: CascadePair) {
    busyPair = pair.id;
    errorMessage = '';
    latestApply = null;
    try {
      latestPlan = await apiPost<CascadeConfigPlanResult>(`/api/cascade/pairs/${pair.id}/plan`, {});
      await loadCascade();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busyPair = '';
    }
  }

  async function applyConfig(pair: CascadePair) {
    busyPair = pair.id;
    errorMessage = '';
    try {
      latestApply = await apiPost<CascadeConfigApplyResult>(`/api/cascade/pairs/${pair.id}/apply`, {
        confirm: true
      });
      latestPlan = { pair: latestApply.pair, plan: latestApply.plan };
      await loadCascade();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busyPair = '';
    }
  }

  async function sampleHealth() {
    sampling = true;
    errorMessage = '';
    healthMessage = '';
    try {
      const result = await apiPost<CascadeHealthSampleResult>('/api/cascade/health/sample', {});
      nodes = result.nodes ?? [];
      pairs = result.pairs ?? [];
      latestSamples = result.samples ?? [];
      healthMessage = result.job.userMessage;
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      sampling = false;
    }
  }

  async function copyCode() {
    if (!latestCode || !navigator.clipboard) {
      return;
    }
    await navigator.clipboard.writeText(latestCode.code);
    copied = true;
    window.setTimeout(() => {
      copied = false;
    }, 1800);
  }

  function pairFor(node: CascadeNode) {
    return pairs.find((pair) => pair.targetNodeId === node.id || pair.sourceNodeId === node.id);
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : t.failed;
  }

  function statusText(value: string) {
    if ($preferences.language !== 'zh-CN') {
      return value;
    }
    if (value === 'trusted') return '可信';
    if (value === 'disabled') return '已停用';
    if (value === 'pending') return '待确认';
    if (value === 'applied') return '已应用';
    if (value === 'planned') return '已规划';
    if (value === 'pending_apply') return '待应用';
    if (value === 'active') return '运行中';
    return value;
  }
</script>

<svelte:head>
  <title>{t.title} - OMO</title>
  <meta
    name="description"
    content="Manage authorized one-hop OMO cascade node pairing and trust records."
  />
</svelte:head>

{#snippet actions()}
  <button class="icon-button" type="button" aria-label={t.refresh} onclick={loadCascade} disabled={loading}>
    <RefreshCw size={17} class={loading ? 'spin' : ''} />
  </button>
  <button class="icon-button" type="button" aria-label={t.sample} onclick={sampleHealth} disabled={sampling}>
    <Activity size={17} class={sampling ? 'spin' : ''} />
  </button>
{/snippet}

<ConsoleShell title={t.title} eyebrow={t.phase} activeHref="/cascade" {actions}>
    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}

    <section class="summary-grid" aria-label={t.summaryLabel}>
      <article class="metric-card">
        <div class="metric-icon"><Network size={20} /></div>
        <div>
          <p>{t.trustedNodes}</p>
          <strong>{nodes.filter((node) => node.status === 'trusted').length}</strong>
          <span>{nodes.length} {t.totalRecords}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><Link2 size={20} /></div>
        <div>
          <p>{t.oneHopLinks}</p>
          <strong>{pairs.length}</strong>
          <span>{pairs.filter((pair) => pair.configState === 'applied').length} {t.appliedRecords}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><CheckCircle2 size={20} /></div>
        <div>
          <p>{t.online}</p>
          <strong>{nodes.filter((node) => node.online).length}</strong>
          <span>{latestSamples.length} {t.latestSamples}</span>
        </div>
      </article>
    </section>

    {#if healthMessage}
      <p class="success-text">{healthMessage}</p>
    {/if}

    <section class="cascade-grid">
      <form class="panel cascade-form" onsubmit={(event) => { event.preventDefault(); createCode(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">{t.exitNode}</p>
            <h2>{t.createPairingCode}</h2>
          </div>
          <Network size={20} />
        </div>

        <label>
          <span>{t.nodeName}</span>
          <input bind:value={nodeName} maxlength="80" required />
        </label>
        <label>
          <span>{t.domain}</span>
          <input bind:value={domain} placeholder="exit.example.com" required />
        </label>
        <label>
          <span>{t.ttlMinutes}</span>
          <input bind:value={ttlMinutes} min="5" max="60" type="number" />
        </label>

        <button class="primary-action" type="submit" disabled={creating}>
          {#if creating}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <Network size={17} />
          {/if}
          {t.createCode}
        </button>

        {#if latestCode}
          <div class="secret-box cascade-code">
            <span>{t.oneTimePairingCode}</span>
            <code>{latestCode.code}</code>
            <button type="button" onclick={copyCode}>
              <ClipboardCopy size={16} />
              {copied ? t.copied : t.copyCode}
            </button>
          </div>
        {/if}
      </form>

      <form class="panel cascade-form" onsubmit={(event) => { event.preventDefault(); acceptCode(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">{t.entryNode}</p>
            <h2>{t.acceptPairing}</h2>
          </div>
          <Link2 size={20} />
        </div>

        <label>
          <span>{t.exitDomain}</span>
          <input bind:value={exitDomain} placeholder="exit.example.com" required />
        </label>
        <label>
          <span>{t.pairingCode}</span>
          <textarea bind:value={pairingCode} rows="6" required></textarea>
        </label>

        <button class="primary-action" type="submit" disabled={accepting}>
          {#if accepting}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <Link2 size={17} />
          {/if}
          {t.acceptPairing}
        </button>
      </form>
    </section>

    {#if latestPlan}
      <section class="service-section">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">{t.configurationPlan}</p>
            <h2>{t.oneHopPreview}</h2>
          </div>
          <ClipboardList size={20} />
        </div>
        <div class="secret-box cascade-code">
          <span>{latestPlan.plan.summary}</span>
          <code>{latestPlan.plan.configPath}</code>
          {#if latestPlan.plan.warnings.length > 0}
            <p class="empty-text">{latestPlan.plan.warnings.join(' ')}</p>
          {/if}
        </div>
        {#if latestApply}
          <p class="success-text">{latestApply.job.userMessage}</p>
        {/if}
      </section>
    {/if}

    <section class="service-section">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.trustRecords}</p>
          <h2>{t.knownNodes}</h2>
        </div>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>{t.loadingNodes}</span>
        </div>
      {:else if nodes.length === 0}
        <p class="empty-text">{t.emptyNodes}</p>
      {:else}
        <div class="subscription-list">
          {#each nodes as node}
            {@const pair = pairFor(node)}
            <article class="subscription-row cascade-row">
              <div>
                <h3>{node.name}</h3>
                <p>{node.domain} - {statusText(node.status)} - {statusText(pair?.configState ?? t.noLinkRecord)}</p>
                <p>
                  {node.online ? t.online : t.offline} - {node.latencyMs} ms - {node.throughputMbps.toFixed(3)} Mbps
                  {#if node.lastError}
                    - {node.lastError}
                  {/if}
                </p>
                {#if node.trustKeyFingerprint}
                  <code>{node.trustKeyFingerprint}</code>
                {/if}
              </div>
              <div class="row-actions">
                {#if pair}
                  <button type="button" onclick={() => planConfig(pair)} disabled={busyPair !== '' || busyNode !== ''}>
                    <ClipboardList size={16} />
                    {t.plan}
                  </button>
                  <button type="button" onclick={() => applyConfig(pair)} disabled={busyPair !== '' || busyNode !== '' || node.status === 'disabled'}>
                    <CheckCircle2 size={16} />
                    {t.apply}
                  </button>
                {/if}
                <button type="button" onclick={() => disableNode(node)} disabled={busyNode !== ''}>
                  <ShieldCheck size={16} />
                  {node.status === 'disabled' ? t.trust : t.disable}
                </button>
                <button type="button" onclick={() => deleteNode(node)} disabled={busyNode !== ''}>
                  <Trash2 size={16} />
                  {t.delete}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>
</ConsoleShell>
