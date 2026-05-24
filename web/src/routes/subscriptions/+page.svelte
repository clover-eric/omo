<script lang="ts">
  import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy';
  import KeyRound from '@lucide/svelte/icons/key-round';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import Network from '@lucide/svelte/icons/network';
  import QrCode from '@lucide/svelte/icons/qr-code';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import { onMount } from 'svelte';
  import {
    apiGet,
    apiPost,
    type SubscriptionList,
    type SubscriptionToken,
    type SubscriptionTokenResult
  } from '$lib/api';

  let subscriptions = $state<SubscriptionToken[]>([]);
  let name = $state('Operations devices');
  let expiresAt = $state('');
  let loading = $state(true);
  let submitting = $state(false);
  let rotatingId = $state('');
  let errorMessage = $state('');
  let latestToken = $state<SubscriptionTokenResult | null>(null);
  let copied = $state('');

  onMount(() => {
    void loadSubscriptions();
  });

  async function loadSubscriptions() {
    loading = true;
    errorMessage = '';
    try {
      const result = await apiGet<SubscriptionList>('/api/subscriptions');
      subscriptions = result.subscriptions;
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      loading = false;
    }
  }

  async function createSubscription() {
    submitting = true;
    errorMessage = '';
    try {
      latestToken = await apiPost<SubscriptionTokenResult>('/api/subscriptions', {
        name,
        expiresAt: expiresAt ? new Date(expiresAt).toISOString() : ''
      });
      await loadSubscriptions();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      submitting = false;
    }
  }

  async function rotateSubscription(subscription: SubscriptionToken) {
    rotatingId = subscription.id;
    errorMessage = '';
    try {
      latestToken = await apiPost<SubscriptionTokenResult>(
        `/api/subscriptions/${subscription.id}/rotate`,
        {}
      );
      await loadSubscriptions();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      rotatingId = '';
    }
  }

  async function copyText(value: string, label: string) {
    if (!navigator.clipboard) {
      copied = '';
      return;
    }
    await navigator.clipboard.writeText(value);
    copied = label;
    window.setTimeout(() => {
      copied = '';
    }, 1800);
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : 'Subscription operation failed.';
  }
</script>

<svelte:head>
  <title>Configuration Distribution - OMO Boundary Operations</title>
  <meta
    name="description"
    content="Manage authorized OMO configuration distribution tokens and QR import output."
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
      <a class="active" href="/subscriptions">
        <QrCode size={18} strokeWidth={1.8} />
        <span>Distribution</span>
      </a>
      <a href="/cascade">
        <Network size={18} strokeWidth={1.8} />
        <span>Cascade Nodes</span>
      </a>
      <a href="/diagnostics">
        <ShieldCheck size={18} strokeWidth={1.8} />
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
        <p class="eyebrow">Phase 4</p>
        <h1>Configuration Distribution</h1>
      </div>
      <button class="icon-button" type="button" aria-label="Refresh subscriptions" onclick={loadSubscriptions} disabled={loading}>
        <RefreshCw size={17} class={loading ? 'spin' : ''} />
      </button>
    </header>

    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}

    <section class="distribution-grid">
      <form class="panel distribution-form" onsubmit={(event) => { event.preventDefault(); createSubscription(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">New Token</p>
            <h2>Authorized Import Link</h2>
          </div>
          <KeyRound size={20} />
        </div>

        <label>
          <span>Name</span>
          <input bind:value={name} maxlength="80" required />
        </label>

        <label>
          <span>Expiration</span>
          <input bind:value={expiresAt} type="datetime-local" />
        </label>

        <button type="submit" disabled={submitting}>
          {#if submitting}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <KeyRound size={17} />
          {/if}
          Create Token
        </button>
      </form>

      <section class="panel issued-token" aria-label="Latest issued token">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">One-Time Secret</p>
            <h2>Latest Token</h2>
          </div>
          <QrCode size={20} />
        </div>

        {#if latestToken}
          <div class="secret-box">
            <span>Import URL</span>
            <code>{latestToken.url}</code>
            <button type="button" onclick={() => copyText(latestToken.url, 'url')}>
              <ClipboardCopy size={16} />
              {copied === 'url' ? 'Copied' : 'Copy URL'}
            </button>
          </div>

          <div class="format-links">
            <a href={`${latestToken.url}?format=sing-box`}>sing-box JSON</a>
            <a href={`${latestToken.url}?format=clash`}>Clash/Mihomo</a>
            <a href={`${latestToken.url}?format=uri`}>Direct URL</a>
            <a href={`${latestToken.url}?format=qr`}>QR SVG</a>
          </div>

          <img class="qr-preview" src={`${latestToken.url}?format=qr`} alt="Subscription QR code" />
        {:else}
          <p class="empty-text">Create or rotate a token to reveal the one-time import URL.</p>
        {/if}
      </section>
    </section>

    <section class="service-section">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Active Records</p>
          <h2>Smart Subscriptions</h2>
        </div>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>Loading subscription records...</span>
        </div>
      {:else if subscriptions.length === 0}
        <p class="empty-text">No distribution tokens have been created yet.</p>
      {:else}
        <div class="subscription-list">
          {#each subscriptions as subscription}
            <article class="subscription-row">
              <div>
                <h3>{subscription.name}</h3>
                <p>{subscription.status} - created {new Date(subscription.createdAt).toLocaleString()}</p>
              </div>
              <button
                type="button"
                onclick={() => rotateSubscription(subscription)}
                disabled={rotatingId !== ''}
              >
                {#if rotatingId === subscription.id}
                  <LoaderCircle size={16} class="spin" />
                {:else}
                  <RefreshCw size={16} />
                {/if}
                Rotate
              </button>
            </article>
          {/each}
        </div>
      {/if}
    </section>
  </main>
</div>
