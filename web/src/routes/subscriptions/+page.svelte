<script lang="ts">
  import Ban from '@lucide/svelte/icons/ban';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardCopy from '@lucide/svelte/icons/clipboard-copy';
  import Eye from '@lucide/svelte/icons/eye';
  import KeyRound from '@lucide/svelte/icons/key-round';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import QrCode from '@lucide/svelte/icons/qr-code';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import Trash2 from '@lucide/svelte/icons/trash-2';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { formatDateTime } from '$lib/format';
  import { localizedErrorMessage } from '$lib/localizedErrors';
  import { preferences, type Language } from '$lib/preferences';
  import {
    apiDelete,
    apiGet,
    apiPatch,
    apiPost,
    type SubscriptionDeleteResult,
    type SubscriptionList,
    type SubscriptionToken,
    type SubscriptionTokenResult,
    type SubscriptionUpdateResult
  } from '$lib/api';

  type Copy = {
    title: string;
    phase: string;
    refresh: string;
    newToken: string;
    createIntro: string;
    name: string;
    defaultTokenName: string;
    expiration: string;
    expirationPlaceholder: string;
    createToken: string;
    manageTitle: string;
    manageEyebrow: string;
    noSelection: string;
    oneTimeUrl: string;
    hiddenUrl: string;
    rotateToReveal: string;
    importUrl: string;
    copied: string;
    copyUrl: string;
    activeRecords: string;
    distributionEntries: string;
    loading: string;
    empty: string;
    rotate: string;
    revealNew: string;
    created: string;
    updated: string;
    expires: string;
    neverExpires: string;
    active: string;
    disabled: string;
    manage: string;
    disable: string;
    enable: string;
    delete: string;
    confirmDelete: string;
    cancelDelete: string;
    formats: string;
    qrPreview: string;
    publicDisabled: string;
    operationFailed: string;
    invalidExpiration: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '配置分发',
      phase: '第四阶段',
      refresh: '刷新配置分发记录',
      newToken: '新分发入口',
      createIntro: '为一组授权设备创建独立导入入口，后续可轮换、禁用或删除。',
      name: '名称',
      defaultTokenName: '运维设备',
      expiration: '过期时间',
      expirationPlaceholder: '可留空，例如 2026-06-01 09:30',
      createToken: '创建入口',
      manageTitle: '分发入口管理',
      manageEyebrow: '选中记录',
      noSelection: '选择一条分发记录后，可以管理状态并轮换新的导入 URL。',
      oneTimeUrl: '新链接已显示',
      hiddenUrl: '历史导入 URL 已隐藏',
      rotateToReveal: '为保护分发令牌，旧链接不会再次显示。轮换后会立即显示新的导入 URL。',
      importUrl: '导入 URL',
      copied: '已复制',
      copyUrl: '复制 URL',
      activeRecords: '分发记录',
      distributionEntries: '智能订阅入口',
      loading: '正在加载配置分发记录...',
      empty: '尚未创建配置分发入口。',
      rotate: '轮换',
      revealNew: '轮换并显示新链接',
      created: '创建',
      updated: '更新',
      expires: '过期',
      neverExpires: '长期有效',
      active: '启用',
      disabled: '禁用',
      manage: '管理',
      disable: '禁用',
      enable: '启用',
      delete: '删除',
      confirmDelete: '确认删除',
      cancelDelete: '取消',
      formats: '可用导入格式',
      qrPreview: '二维码导入',
      publicDisabled: '该入口已禁用，公开导入地址不会返回配置。',
      operationFailed: '配置分发操作失败。',
      invalidExpiration: '过期时间格式无效，请使用 2026-06-01 09:30。'
    },
    'en-US': {
      title: 'Configuration Distribution',
      phase: 'Phase 4',
      refresh: 'Refresh distribution records',
      newToken: 'New Distribution Entry',
      createIntro: 'Create an import entry for authorized devices, then rotate, disable, or delete it later.',
      name: 'Name',
      defaultTokenName: 'Operations devices',
      expiration: 'Expiration',
      expirationPlaceholder: 'Optional, for example 2026-06-01 09:30',
      createToken: 'Create Entry',
      manageTitle: 'Distribution Entry Management',
      manageEyebrow: 'Selected record',
      noSelection: 'Select a distribution record to manage status and rotate a new import URL.',
      oneTimeUrl: 'New link revealed',
      hiddenUrl: 'Historical import URL hidden',
      rotateToReveal: 'For token safety, old links are not displayed again. Rotate to reveal a new import URL immediately.',
      importUrl: 'Import URL',
      copied: 'Copied',
      copyUrl: 'Copy URL',
      activeRecords: 'Distribution Records',
      distributionEntries: 'Smart Subscription Entries',
      loading: 'Loading distribution records...',
      empty: 'No distribution entries have been created yet.',
      rotate: 'Rotate',
      revealNew: 'Rotate and reveal new link',
      created: 'Created',
      updated: 'Updated',
      expires: 'Expires',
      neverExpires: 'No expiration',
      active: 'Active',
      disabled: 'Disabled',
      manage: 'Manage',
      disable: 'Disable',
      enable: 'Enable',
      delete: 'Delete',
      confirmDelete: 'Confirm delete',
      cancelDelete: 'Cancel',
      formats: 'Available import formats',
      qrPreview: 'QR import',
      publicDisabled: 'This entry is disabled; the public import address will not return configuration.',
      operationFailed: 'Configuration distribution operation failed.',
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
  let busyId = $state('');
  let selectedId = $state('');
  let deleteConfirmId = $state('');
  let errorMessage = $state('');
  let latestToken = $state<SubscriptionTokenResult | null>(null);
  let copied = $state('');
  let t = $derived(copy[$preferences.language]);
  let selectedSubscription = $derived(
    subscriptions.find((subscription) => subscription.id === selectedId) ?? subscriptions[0] ?? null
  );
  let selectedTokenVisible = $derived(
    latestToken && selectedSubscription && latestToken.subscription.id === selectedSubscription.id
      ? latestToken
      : null
  );

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

  $effect(() => {
    if (subscriptions.length === 0) {
      selectedId = '';
      return;
    }
    if (!selectedId || !subscriptions.some((subscription) => subscription.id === selectedId)) {
      selectedId = subscriptions[0].id;
    }
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
      selectedId = latestToken.subscription.id;
      await loadSubscriptions();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      submitting = false;
    }
  }

  async function rotateSubscription(subscription: SubscriptionToken) {
    busyId = `${subscription.id}:rotate`;
    errorMessage = '';
    try {
      latestToken = await apiPost<SubscriptionTokenResult>(
        `/api/subscriptions/${subscription.id}/rotate`,
        {}
      );
      selectedId = subscription.id;
      await loadSubscriptions();
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busyId = '';
    }
  }

  async function setSubscriptionStatus(subscription: SubscriptionToken, status: SubscriptionToken['status']) {
    busyId = `${subscription.id}:status`;
    errorMessage = '';
    try {
      const result = await apiPatch<SubscriptionUpdateResult>(`/api/subscriptions/${subscription.id}`, {
        status
      });
      upsertSubscription(result.subscription);
      if (latestToken?.subscription.id === subscription.id && status === 'disabled') {
        latestToken = null;
      }
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busyId = '';
    }
  }

  async function deleteSubscription(subscription: SubscriptionToken) {
    if (deleteConfirmId !== subscription.id) {
      deleteConfirmId = subscription.id;
      return;
    }
    busyId = `${subscription.id}:delete`;
    errorMessage = '';
    try {
      await apiDelete<SubscriptionDeleteResult>(`/api/subscriptions/${subscription.id}`);
      subscriptions = subscriptions.filter((item) => item.id !== subscription.id);
      if (latestToken?.subscription.id === subscription.id) {
        latestToken = null;
      }
      deleteConfirmId = '';
    } catch (error) {
      errorMessage = messageFrom(error);
    } finally {
      busyId = '';
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

  function upsertSubscription(subscription: SubscriptionToken) {
    const byId = new Map(subscriptions.map((item) => [item.id, item]));
    byId.set(subscription.id, subscription);
    subscriptions = Array.from(byId.values()).sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  }

  function statusText(status: SubscriptionToken['status']) {
    return status === 'active' ? t.active : t.disabled;
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
          <h2>{t.createToken}</h2>
          <p class="panel-note">{t.createIntro}</p>
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

    <section class="panel issued-token" aria-label={t.manageTitle}>
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.manageEyebrow}</p>
          <h2>{selectedSubscription ? selectedSubscription.name : t.manageTitle}</h2>
          {#if selectedSubscription}
            <p class="panel-note">
              {statusText(selectedSubscription.status)} / {t.updated}
              {formatDateTime(selectedSubscription.updatedAt, $preferences.language)}
            </p>
          {/if}
        </div>
        <QrCode size={20} />
      </div>

      {#if !selectedSubscription}
        <p class="empty-text">{t.noSelection}</p>
      {:else}
        {#if selectedSubscription.status === 'disabled'}
          <p class="warning-text">{t.publicDisabled}</p>
        {/if}

        {#if selectedTokenVisible}
          <div class="secret-box">
            <span>{t.oneTimeUrl}</span>
            <code>{selectedTokenVisible.url}</code>
            <button type="button" onclick={() => copyText(selectedTokenVisible.url, 'url')}>
              <ClipboardCopy size={16} />
              {copied === 'url' ? t.copied : t.copyUrl}
            </button>
          </div>

          <p class="eyebrow format-heading">{t.formats}</p>
          <div class="format-links">
            <a href={`${selectedTokenVisible.url}?format=sing-box`}>sing-box JSON</a>
            <a href={`${selectedTokenVisible.url}?format=clash`}>Clash/Mihomo</a>
            <a href={`${selectedTokenVisible.url}?format=uri`}>Direct URL</a>
            <a href={`${selectedTokenVisible.url}?format=qr`}>QR SVG</a>
          </div>

          <img class="qr-preview" src={`${selectedTokenVisible.url}?format=qr`} alt={t.qrPreview} />
        {:else}
          <div class="secret-placeholder">
            <KeyRound size={22} />
            <strong>{t.hiddenUrl}</strong>
            <p>{t.rotateToReveal}</p>
            <button
              type="button"
              onclick={() => rotateSubscription(selectedSubscription)}
              disabled={busyId !== '' || selectedSubscription.status !== 'active'}
            >
              {#if busyId === `${selectedSubscription.id}:rotate`}
                <LoaderCircle size={16} class="spin" />
              {:else}
                <RefreshCw size={16} />
              {/if}
              {t.revealNew}
            </button>
          </div>
        {/if}
      {/if}
    </section>
  </section>

  <section class="service-section">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">{t.activeRecords}</p>
        <h2>{t.distributionEntries}</h2>
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
          <article class:active-row={selectedSubscription?.id === subscription.id} class="subscription-row">
            <button
              class="row-select"
              type="button"
              onclick={() => {
                selectedId = subscription.id;
                deleteConfirmId = '';
              }}
            >
              <Eye size={16} />
              {t.manage}
            </button>
            <div class="subscription-main">
              <h3>{subscription.name}</h3>
              <p>
                {statusText(subscription.status)} / {t.created}
                {formatDateTime(subscription.createdAt, $preferences.language)}
              </p>
              <p>
                {subscription.expiresAt
                  ? `${t.expires} ${formatDateTime(subscription.expiresAt, $preferences.language)}`
                  : t.neverExpires}
              </p>
            </div>
            <div class="row-actions">
              <button
                type="button"
                onclick={() => rotateSubscription(subscription)}
                disabled={busyId !== '' || subscription.status !== 'active'}
              >
                {#if busyId === `${subscription.id}:rotate`}
                  <LoaderCircle size={16} class="spin" />
                {:else}
                  <RefreshCw size={16} />
                {/if}
                {t.rotate}
              </button>
              {#if subscription.status === 'active'}
                <button
                  class="secondary-action"
                  type="button"
                  onclick={() => setSubscriptionStatus(subscription, 'disabled')}
                  disabled={busyId !== ''}
                >
                  <Ban size={16} />
                  {t.disable}
                </button>
              {:else}
                <button
                  class="secondary-action"
                  type="button"
                  onclick={() => setSubscriptionStatus(subscription, 'active')}
                  disabled={busyId !== ''}
                >
                  <CheckCircle2 size={16} />
                  {t.enable}
                </button>
              {/if}
              <button
                class="danger-action"
                type="button"
                onclick={() => deleteSubscription(subscription)}
                disabled={busyId !== ''}
              >
                <Trash2 size={16} />
                {deleteConfirmId === subscription.id ? t.confirmDelete : t.delete}
              </button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</ConsoleShell>
