<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { localizedErrorMessage } from '$lib/localizedErrors';
  import { preferences, type Language } from '$lib/preferences';
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

  type Copy = {
    title: string;
    eyebrow: string;
    refresh: string;
    failed: string;
    settingsSaved: string;
    updateRefreshed: string;
    summaryLabel: string;
    currentVersion: string;
    update: string;
    backups: string;
    provider: string;
    updateNotLoaded: string;
    updateAvailable: string;
    updateCurrent: string;
    noUpdateResult: string;
    readyArchives: string;
    providerEnabled: string;
    providerDisabled: string;
    releaseChannel: string;
    updateManifest: string;
    manifestUrl: string;
    saveSettings: string;
    checkUpdate: string;
    latest: string;
    notConfigured: string;
    channel: string;
    defaultChannel: string;
    artifact: string;
    noMatchingArtifact: string;
    checksum: string;
    notReported: string;
    confirmApply: string;
    confirmRollback: string;
    applyUpdate: string;
    rollback: string;
    diagnostics: string;
    optionalProvider: string;
    enableProvider: string;
    name: string;
    endpoint: string;
    timeout: string;
    apiKey: string;
    savedCredential: string;
    optionalCredential: string;
    clearCredential: string;
    saveProvider: string;
    backupRestore: string;
    encryptedBackupRecords: string;
    createBackup: string;
    loadingBackups: string;
    emptyBackups: string;
    confirm: string;
    restore: string;
    backupRecord: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '设置',
      eyebrow: '运维设置',
      refresh: '刷新设置',
      failed: '设置操作失败。',
      settingsSaved: '设置已保存。',
      updateRefreshed: '更新状态已刷新。',
      summaryLabel: '设置概览',
      currentVersion: '当前版本',
      update: '更新',
      backups: '备份',
      provider: '提供方',
      updateNotLoaded: '尚未读取更新状态。',
      updateAvailable: '可更新',
      updateCurrent: '当前版本',
      noUpdateResult: '暂无更新检查结果。',
      readyArchives: '条可用归档记录。',
      providerEnabled: '已启用',
      providerDisabled: '已停用',
      releaseChannel: '发布通道',
      updateManifest: '更新清单',
      manifestUrl: 'HTTPS 清单 URL',
      saveSettings: '保存设置',
      checkUpdate: '检查更新',
      latest: '最新版本',
      notConfigured: '未配置',
      channel: '通道',
      defaultChannel: '默认',
      artifact: '制品',
      noMatchingArtifact: '无匹配制品',
      checksum: '校验和',
      notReported: '未报告',
      confirmApply: '确认应用更新',
      confirmRollback: '确认回滚更新',
      applyUpdate: '应用更新',
      rollback: '回滚',
      diagnostics: '诊断',
      optionalProvider: '可选提供方',
      enableProvider: '服务器体检时启用',
      name: '名称',
      endpoint: 'HTTPS 端点',
      timeout: '超时',
      apiKey: 'API 密钥',
      savedCredential: '已保存凭据',
      optionalCredential: '可选凭据',
      clearCredential: '清除已保存凭据',
      saveProvider: '保存提供方',
      backupRestore: '备份与恢复',
      encryptedBackupRecords: '加密备份记录',
      createBackup: '创建备份',
      loadingBackups: '正在加载备份记录...',
      emptyBackups: '尚未创建备份记录。',
      confirm: '确认',
      restore: '恢复',
      backupRecord: '备份'
    },
    'en-US': {
      title: 'Settings',
      eyebrow: 'Operations',
      refresh: 'Refresh settings',
      failed: 'Settings operation failed.',
      settingsSaved: 'Settings saved.',
      updateRefreshed: 'Update status refreshed.',
      summaryLabel: 'Settings summary',
      currentVersion: 'Current version',
      update: 'Update',
      backups: 'Backups',
      provider: 'Provider',
      updateNotLoaded: 'Update status not loaded.',
      updateAvailable: 'Available',
      updateCurrent: 'Current',
      noUpdateResult: 'No update check result.',
      readyArchives: 'ready archive records.',
      providerEnabled: 'Enabled',
      providerDisabled: 'Disabled',
      releaseChannel: 'Release Channel',
      updateManifest: 'Update Manifest',
      manifestUrl: 'HTTPS manifest URL',
      saveSettings: 'Save Settings',
      checkUpdate: 'Check Update',
      latest: 'Latest',
      notConfigured: 'Not configured',
      channel: 'Channel',
      defaultChannel: 'Default',
      artifact: 'Artifact',
      noMatchingArtifact: 'No matching artifact',
      checksum: 'Checksum',
      notReported: 'Not reported',
      confirmApply: 'Confirm update apply',
      confirmRollback: 'Confirm update rollback',
      applyUpdate: 'Apply Update',
      rollback: 'Rollback',
      diagnostics: 'Diagnostics',
      optionalProvider: 'Optional Provider',
      enableProvider: 'Enable during server checkup',
      name: 'Name',
      endpoint: 'HTTPS endpoint',
      timeout: 'Timeout',
      apiKey: 'API key',
      savedCredential: 'Saved credential is configured',
      optionalCredential: 'Optional credential',
      clearCredential: 'Clear saved credential',
      saveProvider: 'Save Provider',
      backupRestore: 'Backup And Restore',
      encryptedBackupRecords: 'Encrypted Backup Records',
      createBackup: 'Create Backup',
      loadingBackups: 'Loading backup records...',
      emptyBackups: 'No backup records have been created yet.',
      confirm: 'Confirm',
      restore: 'Restore',
      backupRecord: 'backup'
    }
  };

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
  let t = $derived(copy[$preferences.language]);

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
    backups = result.backups ?? [];
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
      successMessage = t.settingsSaved;
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
      successMessage = t.updateRefreshed;
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
    return localizedErrorMessage(error, $preferences.language, t.failed);
  }
</script>

<svelte:head>
  <title>{t.title} - OMO</title>
  <meta
    name="description"
    content="Manage OMO backup, update, and diagnostics provider settings."
  />
</svelte:head>

{#snippet actions()}
  <button class="icon-button" type="button" aria-label={t.refresh} onclick={loadSettingsPage} disabled={loading}>
    <RefreshCw size={17} class={loading ? 'spin' : ''} />
  </button>
{/snippet}

<ConsoleShell title={t.title} eyebrow={t.eyebrow} activeHref="/settings" {actions}>
    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}
    {#if successMessage}
      <p class="success-text">{successMessage}</p>
    {/if}

    <section class="summary-grid" aria-label={t.summaryLabel}>
      <article class="metric-card">
        <div class="metric-icon"><RefreshCw size={20} /></div>
        <div>
          <p>{t.currentVersion}</p>
          <strong>{updateCheck?.currentVersion ?? '--'}</strong>
          <span>{updateCheck?.platform ?? t.updateNotLoaded}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><CheckCircle2 size={20} /></div>
        <div>
          <p>{t.update}</p>
          <strong>{updateCheck?.updateAvailable ? t.updateAvailable : t.updateCurrent}</strong>
          <span>{updateCheck?.summary ?? t.noUpdateResult}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><ClipboardList size={20} /></div>
        <div>
          <p>{t.backups}</p>
          <strong>{backups.length}</strong>
          <span>{backups.filter((item) => item.status === 'ready').length} {t.readyArchives}</span>
        </div>
      </article>
      <article class="metric-card">
        <div class="metric-icon"><TriangleAlert size={20} /></div>
        <div>
          <p>{t.provider}</p>
          <strong>{providerEnabled ? t.providerEnabled : t.providerDisabled}</strong>
          <span>{providerName}</span>
        </div>
      </article>
    </section>

    <section class="settings-grid">
      <form class="panel provider-form" onsubmit={(event) => { event.preventDefault(); saveSettings(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">{t.releaseChannel}</p>
            <h2>{t.updateManifest}</h2>
          </div>
          <RefreshCw size={20} />
        </div>

        <label>
          <span>{t.manifestUrl}</span>
          <input bind:value={updateManifestUrl} placeholder="https://updates.example/manifest.json" />
        </label>

        <div class="row-actions">
          <button class="primary-action" type="submit" disabled={busy !== ''}>
            {#if busy === 'settings'}
              <LoaderCircle size={17} class="spin" />
            {:else}
              <SlidersHorizontal size={17} />
            {/if}
            {t.saveSettings}
          </button>
          <button class="primary-action secondary-action" type="button" onclick={refreshUpdate} disabled={busy !== ''}>
            <RefreshCw size={17} class={busy === 'update-check' ? 'spin' : ''} />
            {t.checkUpdate}
          </button>
        </div>

        {#if updateCheck}
          <dl class="snapshot-list">
            <div><dt>{t.latest}</dt><dd>{updateCheck.latestVersion ?? t.notConfigured}</dd></div>
            <div><dt>{t.channel}</dt><dd>{updateCheck.channel ?? t.defaultChannel}</dd></div>
            <div><dt>{t.artifact}</dt><dd>{updateCheck.artifactUrl ?? t.noMatchingArtifact}</dd></div>
            <div><dt>{t.checksum}</dt><dd>{updateCheck.checksumSha256 ?? t.notReported}</dd></div>
          </dl>
        {/if}

        <label class="toggle-row">
          <input bind:checked={applyConfirmed} type="checkbox" />
          <span>{t.confirmApply}</span>
        </label>
        <label class="toggle-row">
          <input bind:checked={rollbackConfirmed} type="checkbox" />
          <span>{t.confirmRollback}</span>
        </label>

        <div class="row-actions">
          <button class="primary-action" type="button" onclick={applyUpdate} disabled={busy !== '' || !applyConfirmed}>
            <CheckCircle2 size={17} />
            {t.applyUpdate}
          </button>
          <button class="primary-action danger-action" type="button" onclick={rollbackUpdate} disabled={busy !== '' || !rollbackConfirmed}>
            <RefreshCw size={17} />
            {t.rollback}
          </button>
        </div>

        {#if updateJob}
          <p class="empty-text">{updateJob.job.userMessage}</p>
        {/if}
      </form>

      <form class="panel provider-form" onsubmit={(event) => { event.preventDefault(); saveSettings(); }}>
        <div class="panel-heading">
          <div>
            <p class="eyebrow">{t.diagnostics}</p>
            <h2>{t.optionalProvider}</h2>
          </div>
          <Activity size={20} />
        </div>

        <label class="toggle-row">
          <input bind:checked={providerEnabled} type="checkbox" />
          <span>{t.enableProvider}</span>
        </label>
        <label>
          <span>{t.name}</span>
          <input bind:value={providerName} maxlength="80" />
        </label>
        <label>
          <span>{t.endpoint}</span>
          <input bind:value={providerEndpoint} placeholder="https://provider.example/check" />
        </label>
        <label>
          <span>{t.timeout}</span>
          <input bind:value={providerTimeout} min="1" max="10" type="number" />
        </label>
        <label>
          <span>{t.apiKey}</span>
          <input bind:value={providerApiKey} placeholder={providerKeyConfigured ? t.savedCredential : t.optionalCredential} type="password" />
        </label>
        <label class="toggle-row">
          <input bind:checked={clearProviderKey} type="checkbox" />
          <span>{t.clearCredential}</span>
        </label>
        <button class="primary-action" type="submit" disabled={busy !== ''}>
          <SlidersHorizontal size={17} />
          {t.saveProvider}
        </button>
      </form>
    </section>

    <section class="panel service-section">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">{t.backupRestore}</p>
          <h2>{t.encryptedBackupRecords}</h2>
        </div>
        <button class="primary-action" type="button" onclick={createBackup} disabled={busy !== ''}>
          {#if busy === 'backup'}
            <LoaderCircle size={17} class="spin" />
          {:else}
            <ClipboardList size={17} />
          {/if}
          {t.createBackup}
        </button>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>{t.loadingBackups}</span>
        </div>
      {:else if backups.length === 0}
        <p class="empty-text">{t.emptyBackups}</p>
      {:else}
        <div class="subscription-list">
          {#each backups as backup}
            <article class="subscription-row">
              <div>
                <h3>{backup.status} {t.backupRecord}</h3>
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
                  <span>{t.confirm}</span>
                </label>
                <button
                  class="primary-action danger-action"
                  type="button"
                  onclick={() => restoreBackup(backup)}
                  disabled={busy !== '' || restoreConfirmId !== backup.id || backup.status !== 'ready'}
                >
                  <RefreshCw size={16} />
                  {t.restore}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>
</ConsoleShell>
