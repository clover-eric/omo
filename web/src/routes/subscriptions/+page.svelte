<script lang="ts">
  import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy';
  import KeyRound from '@lucide/svelte/icons/key-round';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import QrCode from '@lucide/svelte/icons/qr-code';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { formatDateTime } from '$lib/format';
  import { localizedErrorMessage } from '$lib/localizedErrors';
  import { preferences, type Language } from '$lib/preferences';
  import {
    apiGet,
    apiPost,
    type SubscriptionList,
    type SubscriptionToken,
    type SubscriptionTokenResult
  } from '$lib/api';

  type Copy = {
    title: string;
    phase: string;
    refresh: string;
    newToken: string;
    importLink: string;
    name: string;
    defaultTokenName: string;
    expiration: string;
    expirationPlaceholder: string;
    createToken: string;
    oneTimeSecret: string;
    latestToken: string;
    importUrl: string;
    copied: string;
    copyUrl: string;
    noLatest: string;
    activeRecords: string;
    smartSubscriptions: string;
    loading: string;
    empty: string;
    rotate: string;
    created: string;
    operationFailed: string;
    invalidExpiration: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '配置分发',
      phase: '第四阶段',
      refresh: '刷新配置分发记录',
      newToken: '新令牌',
      importLink: '授权导入链接',
      name: '名称',
      defaultTokenName: '运维设备',
      expiration: '过期时间',
      expirationPlaceholder: '可留空，例如 2026-06-01 09:30',
      createToken: '创建令牌',
      oneTimeSecret: '一次性密钥',
      latestToken: '最新令牌',
      importUrl: '导入 URL',
      copied: '已复制',
      copyUrl: '复制 URL',
      noLatest: '创建或轮换令牌后会显示一次性导入 URL。',
      activeRecords: '有效记录',
      smartSubscriptions: '智能订阅',
      loading: '正在加载配置分发记录...',
      empty: '尚未创建配置分发令牌。',
      rotate: '轮换',
      created: '创建于',
      operationFailed: '配置分发操作失败。',
      invalidExpiration: '过期时间格式无效，请使用 2026-06-01 09:30。'
    },
    'en-US': {
      title: 'Configuration Distribution',
      phase: 'Phase 4',
      refresh: 'Refresh distribution records',
      newToken: 'New Token',
      importLink: 'Authorized Import Link',
      name: 'Name',
      defaultTokenName: 'Operations devices',
      expiration: 'Expiration',
      expirationPlaceholder: 'Optional, for example 2026-06-01 09:30',
      createToken: 'Create Token',
      oneTimeSecret: 'One-Time Secret',
      latestToken: 'Latest Token',
      importUrl: 'Import URL',
      copied: 'Copied',
      copyUrl: 'Copy URL',
      noLatest: 'Create or rotate a token to reveal the one-time import URL.',
      activeRecords: 'Active Records',
      smartSubscriptions: 'Smart Subscriptions',
      loading: 'Loading distribution records...',
      empty: 'No distribution tokens have been created yet.',
      rotate: 'Rotate',
      created: 'created',
      operationFailed: 'Subscription operation failed.',
      invalidExpiration: 'Use an expiration like 2026-06-01 09:30.'
    }
  };

  let subscriptions = $state<SubscriptionToken[]>([]);
  let name = $state('');
  let nameEdited = $state(false);
  let previousLanguage = $state<Language>('zh-CN');
  let expiresAt = $state('');
  let loading = $state(true);
  let submitting = $state(false);
  let rotatingId = $state('');
  let errorMessage = $state('');
  let latestToken = $state<SubscriptionTokenResult | null>(null);
  let copied = $state('');
  let t = $derived(copy[$preferences.language]);

  $effect(() => {
    const language = $preferences.language;
    const previousDefault = copy[previousLanguage].defaultTokenName;
    const nextDefault = copy[language].defaultTokenName;
    if (!nameEdited || name === '' || name === previousDefault) {
      name = nextDefault;
      nameEdited = false;
    }
    previousLanguage = language;
  });

  onMount(() => {
    void loadSubscriptions();
  });

  async function loadSubscriptions() {
    loading = true;
    errorMessage = '';
    try {
      const result = await apiGet<SubscriptionList>('/api/subscriptions');
      subscriptions = result.subscriptions ?? [];
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      loading = false;
    }
  }

  async function createSubscription() {
    submitting = true;
    errorMessage = '';
    const expiresAtValue = expirationToISOString();
    if (expiresAtValue === null) {
      errorMessage = t.invalidExpiration;
      submitting = false;
      return;
    }
    try {
      latestToken = await apiPost<SubscriptionTokenResult>('/api/subscriptions', {
        name,
        expiresAt: expiresAtValue
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
    return localizedErrorMessage(error, $preferences.language, t.operationFailed);
  }

  function expirationToISOString() {
    const raw = expiresAt.trim();
    if (!raw) {
      return '';
    }
    const normalized = raw.includes('T') ? raw : raw.replace(' ', 'T');
    if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(normalized)) {
      return null;
    }
    const parsed = new Date(normalized);
    return Number.isNaN(parsed.getTime()) ? null : parsed.toISOString();
  }
</script>

<svelte:head>
  <title>{t.title} - OMO</title>
  <meta
    name="description"
    content="Manage authorized OMO configuration distribution tokens and QR import output."
  />
</svelte:head>

{#snippet actions()}
  <button class="icon-button" type="button" aria-label={t.refresh} onclick={loadSubscriptions} disabled={loading}>
    <RefreshCw size={17} class={loading ? 'spin' : ''} />
  </button>
{/snippet}

<ConsoleShell title={t.title} eyebrow={t.phase} activeHref="/subscriptions" {actions}>
  {#if errorMessage}
    <p class="error-text">{errorMessage}</p>
  {/if}

  <section class="distribution-grid">
    <form class="panel distribution-form" onsubmit={(event) => { event.preventDefault(); createSubscription(); }}>
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.newToken}</p>
          <h2>{t.importLink}</h2>
        </div>
        <KeyRound size={20} />
      </div>

      <label>
        <span>{t.name}</span>
        <input
          value={name}
          maxlength="80"
          required
          oninput={(event) => {
            nameEdited = true;
            name = (event.currentTarget as HTMLInputElement).value;
          }}
        />
      </label>

      <label>
        <span>{t.expiration}</span>
        <input bind:value={expiresAt} inputmode="numeric" placeholder={t.expirationPlaceholder} />
      </label>

      <button type="submit" disabled={submitting}>
        {#if submitting}
          <LoaderCircle size={17} class="spin" />
        {:else}
          <KeyRound size={17} />
        {/if}
        {t.createToken}
      </button>
    </form>

    <section class="panel issued-token" aria-label={t.latestToken}>
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.oneTimeSecret}</p>
          <h2>{t.latestToken}</h2>
        </div>
        <QrCode size={20} />
      </div>

      {#if latestToken}
        <div class="secret-box">
          <span>{t.importUrl}</span>
          <code>{latestToken.url}</code>
          <button type="button" onclick={() => copyText(latestToken.url, 'url')}>
            <ClipboardCopy size={16} />
            {copied === 'url' ? t.copied : t.copyUrl}
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
        <p class="empty-text">{t.noLatest}</p>
      {/if}
    </section>
  </section>

  <section class="service-section">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">{t.activeRecords}</p>
        <h2>{t.smartSubscriptions}</h2>
      </div>
    </div>

    {#if loading}
      <div class="loading-row">
        <LoaderCircle size={18} class="spin" />
        <span>{t.loading}</span>
      </div>
    {:else if subscriptions.length === 0}
      <p class="empty-text">{t.empty}</p>
    {:else}
      <div class="subscription-list">
        {#each subscriptions as subscription}
          <article class="subscription-row">
            <div>
              <h3>{subscription.name}</h3>
              <p>{subscription.status} - {t.created} {formatDateTime(subscription.createdAt, $preferences.language)}</p>
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
              {t.rotate}
            </button>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</ConsoleShell>
