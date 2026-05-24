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
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import { onMount } from 'svelte';
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

  let nodes = $state<CascadeNode[]>([]);
  let pairs = $state<CascadePair[]>([]);
  let nodeName = $state('Exit cascade node');
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

  onMount(() => {
    void loadCascade();
  });

  async function loadCascade() {
    loading = true;
    errorMessage = '';
    try {
      const result = await apiGet<CascadeNodeList>('/api/cascade/nodes');
      nodes = result.nodes;
      pairs = result.pairs;
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
      nodes = result.nodes;
      pairs = result.pairs;
      latestSamples = result.samples;
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
    return error instanceof Error ? error.message : 'Cascade operation failed.';
  }
</script>

<svelte:head>
  <title>Cascade Nodes - OMO Boundary Operations</title>
  <meta
    name="description"
    content="Manage authorized one-hop OMO cascade node pairing and trust records."
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
      <a class="active" href="/cascade">
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
      <a href="/settings">
        <SlidersHorizontal size={18} strokeWidth={1.8} />
        <span>Settings</span>
      </a>
    </nav>
  </aside>

  <main class="workspace">
    <header class="topbar">
      <div>
        <p class="eyebrow">Phase 6</p>
        <h1>Cascade Nodes</h1>
      </div>
      <button class="icon-button" type="button" aria-label="Refresh cascade nodes" onclick={loadCascade} disabled={loading}>
        <RefreshCw size={17} class={loading ? 'spin' : ''} />
      </button>
      <button class="icon-button" type="button" aria-label="Sample cascade health" onclick={sampleHealth} disabled={sampling}>
        <Activity size={17} class={sampling ? 'spin' : ''} />
      </button>
    </header>

    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}

    <section class="summary-grid" aria-label="Cascade summary">
      <article class="metric-card">
        <div class="metric-icon"><Network size={20} /></div>
        <div>
          <p>Trusted nodes</p>
          <strong>{nodes.filter((node) => node.status === 'trusted').length}</strong>
          <span>{nodes.length} total cascade records</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><Link2 size={20} /></div>
        <div>
          <p>One-hop links</p>
          <strong>{pairs.length}</strong>
          <span>{pairs.filter((pair) => pair.configState === 'applied').length} applied configuration records</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><CheckCircle2 size={20} /></div>
        <div>
          <p>Online</p>
          <strong>{nodes.filter((node) => node.online).length}</strong>
          <span>{latestSamples.length} latest health samples</span>
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
            <p class="eyebrow">Exit Node</p>
            <h2>Create Pairing Code</h2>
          </div>
          <Network size={20} />
        </div>

        <label>
          <span>Node name</span>
          <input bind:value={nodeName} maxlength="80" required />
        </label>
        <label>
          <span>Domain</span>
          <input bind:value={domain} placeholder="exit.example.com" required />
        </label>
        <label>
          <span>TTL minutes</span>
          <input bind:value={ttlMinutes} min="5" max="60" type="number" />
        </label>

        <button class="primary-action" type="submit" disabled={creating}>
          {#if creating}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <Network size={17} />
          {/if}
          Create Code
        </button>

        {#if latestCode}
          <div class="secret-box cascade-code">
            <span>One-time pairing code</span>
            <code>{latestCode.code}</code>
            <button type="button" onclick={copyCode}>
              <ClipboardCopy size={16} />
              {copied ? 'Copied' : 'Copy Code'}
            </button>
          </div>
        {/if}
      </form>

      <form class="panel cascade-form" onsubmit={(event) => { event.preventDefault(); acceptCode(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Entry Node</p>
            <h2>Accept Pairing</h2>
          </div>
          <Link2 size={20} />
        </div>

        <label>
          <span>Exit domain</span>
          <input bind:value={exitDomain} placeholder="exit.example.com" required />
        </label>
        <label>
          <span>Pairing code</span>
          <textarea bind:value={pairingCode} rows="6" required></textarea>
        </label>

        <button class="primary-action" type="submit" disabled={accepting}>
          {#if accepting}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <Link2 size={17} />
          {/if}
          Accept Pairing
        </button>
      </form>
    </section>

    {#if latestPlan}
      <section class="service-section">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Configuration Plan</p>
            <h2>One-Hop Cascade Preview</h2>
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
          <p class="eyebrow">Trust Records</p>
          <h2>Known Cascade Nodes</h2>
        </div>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>Loading cascade nodes...</span>
        </div>
      {:else if nodes.length === 0}
        <p class="empty-text">No cascade nodes have been paired yet.</p>
      {:else}
        <div class="subscription-list">
          {#each nodes as node}
            {@const pair = pairFor(node)}
            <article class="subscription-row cascade-row">
              <div>
                <h3>{node.name}</h3>
                <p>{node.domain} - {node.status} - {pair?.configState ?? 'no link record'}</p>
                <p>
                  {node.online ? 'online' : 'offline'} - {node.latencyMs} ms - {node.throughputMbps.toFixed(3)} Mbps
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
                    Plan
                  </button>
                  <button type="button" onclick={() => applyConfig(pair)} disabled={busyPair !== '' || busyNode !== '' || node.status === 'disabled'}>
                    <CheckCircle2 size={16} />
                    Apply
                  </button>
                {/if}
                <button type="button" onclick={() => disableNode(node)} disabled={busyNode !== ''}>
                  <ShieldCheck size={16} />
                  {node.status === 'disabled' ? 'Trust' : 'Disable'}
                </button>
                <button type="button" onclick={() => deleteNode(node)} disabled={busyNode !== ''}>
                  <Trash2 size={16} />
                  Delete
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>
  </main>
</div>
