<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import Boxes from '@lucide/svelte/icons/boxes';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import Network from '@lucide/svelte/icons/network';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import Server from '@lucide/svelte/icons/server';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { localizedErrorMessage } from '$lib/localizedErrors';
  import { preferences, type Language } from '$lib/preferences';
  import { apiGet, type SystemOverview } from '$lib/api';

  type Copy = {
    title: string;
    eyebrow: string;
    refresh: string;
    loading: string;
    loadError: string;
    initialized: string;
    notInitialized: string;
    coreReady: string;
    coreAttention: string;
    systemVersion: string;
    currentVersion: string;
    accessCore: string;
    managedServices: string;
    servicesNote: string;
    profileTemplates: string;
    profilesNote: string;
    quickActions: string;
    serviceLibrary: string;
    serviceLibraryNote: string;
    distribution: string;
    distributionNote: string;
    diagnostics: string;
    diagnosticsNote: string;
    cascade: string;
    cascadeNote: string;
    settings: string;
    settingsNote: string;
    dashboardState: string;
    panelDomain: string;
    latestJob: string;
    noJob: string;
    timestamp: string;
    unknown: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '概览',
      eyebrow: '运维控制台',
      refresh: '刷新概览',
      loading: '正在加载控制台概览...',
      loadError: '无法加载控制台概览。',
      initialized: '已初始化',
      notInitialized: '待初始化',
      coreReady: '核心就绪',
      coreAttention: '核心需处理',
      systemVersion: '系统版本',
      currentVersion: '当前版本',
      accessCore: '接入核心',
      managedServices: '托管服务',
      servicesNote: '后端持久化的服务实例',
      profileTemplates: '服务模板',
      profilesNote: '可规划的默认接入服务',
      quickActions: '常用操作',
      serviceLibrary: '服务库',
      serviceLibraryNote: '规划、应用和回滚默认接入服务。',
      distribution: '配置分发',
      distributionNote: '创建导入链接、轮换令牌和查看二维码。',
      diagnostics: '服务器体检',
      diagnosticsNote: '运行本机状态、入口和证书检查。',
      cascade: '级联节点',
      cascadeNote: '管理授权的一跳节点配对和链路状态。',
      settings: '设置',
      settingsNote: '管理备份、更新和诊断提供方。',
      dashboardState: '面板状态',
      panelDomain: '面板域名',
      latestJob: '最近任务',
      noJob: '暂无任务记录',
      timestamp: '更新时间',
      unknown: '未知'
    },
    'en-US': {
      title: 'Overview',
      eyebrow: 'Operations Console',
      refresh: 'Refresh overview',
      loading: 'Loading console overview...',
      loadError: 'Unable to load console overview.',
      initialized: 'Initialized',
      notInitialized: 'Not initialized',
      coreReady: 'Core ready',
      coreAttention: 'Core attention',
      systemVersion: 'System version',
      currentVersion: 'Current version',
      accessCore: 'Access core',
      managedServices: 'Managed services',
      servicesNote: 'Backend-persisted service instances',
      profileTemplates: 'Profile templates',
      profilesNote: 'Default access services ready to plan',
      quickActions: 'Quick actions',
      serviceLibrary: 'Service Library',
      serviceLibraryNote: 'Plan, apply, and roll back default access services.',
      distribution: 'Distribution',
      distributionNote: 'Create import links, rotate tokens, and view QR output.',
      diagnostics: 'Server Checkup',
      diagnosticsNote: 'Run local status, entry, and certificate checks.',
      cascade: 'Cascade Nodes',
      cascadeNote: 'Manage authorized one-hop node pairing and link state.',
      settings: 'Settings',
      settingsNote: 'Manage backups, updates, and diagnostics providers.',
      dashboardState: 'Panel state',
      panelDomain: 'Panel domain',
      latestJob: 'Latest job',
      noJob: 'No job records yet',
      timestamp: 'Updated',
      unknown: 'Unknown'
    }
  };

  let overview = $state<SystemOverview | null>(null);
  let loading = $state(true);
  let error = $state('');

  let t = $derived(copy[$preferences.language]);
  let coreReady = $derived(Boolean(overview?.core.installed && overview?.core.healthy));
  let initialized = $derived(Boolean(overview?.bootstrap?.initialized));
  let statusLabel = $derived(initialized && coreReady ? t.coreReady : t.coreAttention);
  let metrics = $derived([
    {
      label: t.dashboardState,
      value: initialized ? t.initialized : t.notInitialized,
      note: overview?.bootstrap?.nextRequirement ?? overview?.bootstrap?.state ?? t.unknown,
      icon: ShieldCheck
    },
    {
      label: t.systemVersion,
      value: overview?.version ?? '--',
      note: t.currentVersion,
      icon: Server
    },
    {
      label: t.managedServices,
      value: String(overview?.counts?.services ?? 0),
      note: t.servicesNote,
      icon: Boxes
    },
    {
      label: t.profileTemplates,
      value: String(overview?.counts?.serviceProfiles ?? 0),
      note: t.profilesNote,
      icon: ClipboardList
    }
  ]);

  const quickActions = [
    { title: 'serviceLibrary', note: 'serviceLibraryNote', href: '/services', icon: Boxes },
    { title: 'distribution', note: 'distributionNote', href: '/subscriptions', icon: ClipboardList },
    { title: 'diagnostics', note: 'diagnosticsNote', href: '/diagnostics', icon: Activity },
    { title: 'cascade', note: 'cascadeNote', href: '/cascade', icon: Network },
    { title: 'settings', note: 'settingsNote', href: '/settings', icon: SlidersHorizontal }
  ] as const;

  onMount(() => {
    void loadOverview();
  });

  async function loadOverview() {
    loading = true;
    error = '';
    try {
      overview = await apiGet<SystemOverview>('/api/system/overview');
    } catch (err) {
      error = localizedErrorMessage(err, $preferences.language, t.loadError);
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>{t.title} - OMO</title>
  <meta
    name="description"
    content="OMO operations overview for authorized boundary access and infrastructure management."
  />
</svelte:head>

{#snippet actions()}
  <button class="icon-button" type="button" aria-label={t.refresh} onclick={loadOverview} disabled={loading}>
    <RefreshCw size={17} class={loading ? 'spin' : ''} />
  </button>
{/snippet}

<ConsoleShell
  title={t.title}
  eyebrow={t.eyebrow}
  activeHref="/dashboard"
  {statusLabel}
  statusReady={initialized && coreReady}
  {actions}
>
  {#if error}
    <p class="error-text">{error}</p>
  {/if}

  {#if loading}
    <div class="loading-row">
      <LoaderCircle size={18} class="spin" />
      <span>{t.loading}</span>
    </div>
  {:else}
    <section class="summary-grid" aria-label={t.title}>
      {#each metrics as metric}
        {@const Icon = metric.icon}
        <article class="metric-card">
          <div class="metric-icon">
            <Icon size={20} />
          </div>
          <div>
            <p>{metric.label}</p>
            <strong>{metric.value}</strong>
            <span>{metric.note}</span>
          </div>
        </article>
      {/each}
    </section>

    <section class="settings-grid">
      <section class="panel">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">{t.quickActions}</p>
            <h2>{t.serviceLibrary}</h2>
          </div>
          <CheckCircle2 size={20} />
        </div>

        <div class="subscription-list">
          {#each quickActions as item}
            {@const Icon = item.icon}
            <a class="subscription-row action-link" href={item.href}>
              <div>
                <h3>{t[item.title]}</h3>
                <p>{t[item.note]}</p>
              </div>
              <Icon size={20} />
            </a>
          {/each}
        </div>
      </section>

      <aside class="panel">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">{t.dashboardState}</p>
            <h2>{statusLabel}</h2>
          </div>
          <ShieldCheck size={20} />
        </div>

        <dl class="snapshot-list">
          <div><dt>{t.panelDomain}</dt><dd>{overview?.bootstrap?.domain ?? t.unknown}</dd></div>
          <div><dt>{t.latestJob}</dt><dd>{overview?.bootstrap?.latestJob?.userMessage ?? t.noJob}</dd></div>
          <div><dt>{t.accessCore}</dt><dd>{overview?.core.message ?? t.unknown}</dd></div>
          <div><dt>{t.timestamp}</dt><dd>{overview ? new Date(overview.timestamp).toLocaleString() : t.unknown}</dd></div>
        </dl>
      </aside>
    </section>
  {/if}
</ConsoleShell>
