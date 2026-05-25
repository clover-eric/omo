<script lang="ts">
  import Boxes from '@lucide/svelte/icons/boxes';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
  import Server from '@lucide/svelte/icons/server';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import Zap from '@lucide/svelte/icons/zap';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { preferences, type Language } from '$lib/preferences';
  import {
    apiGet,
    apiPost,
    type ServiceConfigJobResult,
    type ServiceInstance,
    type ServiceList,
    type ServiceProfile,
    type ServiceResult,
    type SingBoxStatus
  } from '$lib/api';

  type Copy = {
    title: string;
    phase: string;
    defaultServices: string;
    accessCore: string;
    ready: string;
    needsAttention: string;
    loadingCore: string;
    profileTemplates: string;
    backendCatalog: string;
    managedServices: string;
    managedByBackend: string;
    operationsMode: string;
    authorized: string;
    authorizedNote: string;
    coreReady: string;
    coreAttention: string;
    refresh: string;
    loadingCatalog: string;
    loadError: string;
    createError: string;
    actionError: string;
    instances: string;
    emptyInstances: string;
    plannedEntry: string;
    transport: string;
    security: string;
    requirements: string;
    domain: string;
    noDomain: string;
    tls: string;
    udp: string;
    plan: string;
    apply: string;
    rollback: string;
    clientCompatibility: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '服务库',
      phase: '第三阶段',
      defaultServices: '默认接入服务',
      accessCore: '接入核心',
      ready: '就绪',
      needsAttention: '需要处理',
      loadingCore: '正在读取核心状态',
      profileTemplates: '服务模板',
      backendCatalog: '后端托管的服务目录',
      managedServices: '托管服务',
      managedByBackend: '计划实例由后端持久化',
      operationsMode: '运维模式',
      authorized: '授权',
      authorizedNote: '仅用于边界接入和授权基础设施管理',
      coreReady: '核心就绪',
      coreAttention: '核心需处理',
      refresh: '刷新服务数据',
      loadingCatalog: '正在加载服务目录...',
      loadError: '无法加载服务库数据。',
      createError: '无法创建托管服务。',
      actionError: '无法执行服务配置操作。',
      instances: '托管服务实例',
      emptyInstances: '暂未创建托管服务实例。',
      plannedEntry: '托管入口',
      transport: '传输',
      security: '安全',
      requirements: '依赖条件',
      domain: '域名',
      noDomain: '无需域名',
      tls: '证书',
      udp: 'UDP',
      plan: '规划',
      apply: '应用',
      rollback: '回滚',
      clientCompatibility: '客户端兼容性'
    },
    'en-US': {
      title: 'Service Library',
      phase: 'Phase 3',
      defaultServices: 'Default Access Services',
      accessCore: 'Access core',
      ready: 'Ready',
      needsAttention: 'Needs attention',
      loadingCore: 'Loading core status',
      profileTemplates: 'Profile templates',
      backendCatalog: 'Backend-owned service catalog',
      managedServices: 'Managed services',
      managedByBackend: 'Planned instances are stored by the backend',
      operationsMode: 'Operations mode',
      authorized: 'Authorized',
      authorizedNote: 'Boundary access and infrastructure management only',
      coreReady: 'Core ready',
      coreAttention: 'Core attention',
      refresh: 'Refresh service data',
      loadingCatalog: 'Loading service catalog...',
      loadError: 'Unable to load service library data.',
      createError: 'Unable to create managed service.',
      actionError: 'Unable to run service configuration action.',
      instances: 'Managed service instances',
      emptyInstances: 'No managed service instances have been created yet.',
      plannedEntry: 'managed entry',
      transport: 'Transport',
      security: 'Security',
      requirements: 'Requirements',
      domain: 'Domain',
      noDomain: 'No domain required',
      tls: 'TLS',
      udp: 'UDP',
      plan: 'Plan',
      apply: 'Apply',
      rollback: 'Rollback',
      clientCompatibility: 'Client compatibility'
    }
  };

  const localizedProfiles: Record<Language, Record<string, { name: string; summary: string }>> = {
    'zh-CN': {
      'standard-secure-access': {
        name: '标准安全接入',
        summary: '适合常规企业边界接入，兼顾稳定性、可维护性和默认安全基线。'
      },
      'high-throughput-access': {
        name: '高吞吐接入',
        summary: '面向大流量业务链路，优先保障吞吐和连接效率。'
      },
      'broad-compatibility-access': {
        name: '广泛兼容接入',
        summary: '优先覆盖更多客户端和网络环境，适合复杂终端接入。'
      },
      'lightweight-fallback-access': {
        name: '轻量备用接入',
        summary: '用于资源有限或临时恢复场景，保持配置简单可回退。'
      },
      'mobile-optimized-access': {
        name: '移动优化接入',
        summary: '面向移动设备和不稳定网络，降低切换和恢复成本。'
      }
    },
    'en-US': {}
  };

  let profiles = $state<ServiceProfile[]>([]);
  let services = $state<ServiceInstance[]>([]);
  let coreStatus = $state<SingBoxStatus | null>(null);
  let loading = $state(true);
  let error = $state('');
  let busyProfile = $state('');
  let lastJob = $state<ServiceConfigJobResult | null>(null);

  let t = $derived(copy[$preferences.language]);
  let coreReady = $derived(Boolean(coreStatus?.installed && coreStatus?.healthy));
  let metrics = $derived([
    { label: t.accessCore, value: coreReady ? t.ready : t.needsAttention, note: coreStatus?.message ?? t.loadingCore, icon: Server },
    { label: t.profileTemplates, value: profiles.length.toString(), note: t.backendCatalog, icon: Boxes },
    { label: t.managedServices, value: services.length.toString(), note: lastJob ? lastJob.job.userMessage : t.managedByBackend, icon: ClipboardList },
    { label: t.operationsMode, value: t.authorized, note: t.authorizedNote, icon: ShieldCheck }
  ]);

  onMount(() => {
    void loadServices();
  });

  async function loadServices() {
    loading = true;
    error = '';
    try {
      const [serviceResult, statusResult] = await Promise.all([
        apiGet<ServiceList>('/api/services'),
        apiGet<SingBoxStatus>('/api/core/singbox/status')
      ]);
      profiles = serviceResult.profiles;
      services = serviceResult.services;
      coreStatus = statusResult;
    } catch (err) {
      error = err instanceof Error ? err.message : t.loadError;
    } finally {
      loading = false;
    }
  }

  async function planProfile(profile: ServiceProfile) {
    busyProfile = `${profile.id}:plan`;
    error = '';
    try {
      const result = await apiPost<ServiceResult>('/api/services', {
        profileId: profile.id,
        displayName: profileName(profile),
        listenPort: 0,
        status: 'planned'
      });
      services = [result.service, ...services];
    } catch (err) {
      error = err instanceof Error ? err.message : t.createError;
    } finally {
      busyProfile = '';
    }
  }

  async function runConfigAction(profile: ServiceProfile, action: 'apply' | 'rollback') {
    busyProfile = `${profile.id}:${action}`;
    error = '';
    try {
      lastJob = await apiPost<ServiceConfigJobResult>(`/api/services/${profile.id}/${action}`, {});
      if (lastJob.instances && lastJob.instances.length > 0) {
        upsertServices(lastJob.instances);
      }
    } catch (err) {
      error = err instanceof Error ? err.message : t.actionError;
    } finally {
      busyProfile = '';
    }
  }

  function upsertServices(nextServices: ServiceInstance[]) {
    const byId = new Map(services.map((service) => [service.id, service]));
    for (const service of nextServices) {
      byId.set(service.id, service);
    }
    services = Array.from(byId.values()).sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  }

  function profileIcon(profile: ServiceProfile) {
    if (profile.category === 'performance') {
      return Zap;
    }
    if (profile.category === 'compatibility') {
      return CheckCircle2;
    }
    return ShieldCheck;
  }

  function profileName(profile: ServiceProfile) {
    return localizedProfiles[$preferences.language][profile.id]?.name ?? profile.displayName;
  }

  function profileSummary(profile: ServiceProfile) {
    return localizedProfiles[$preferences.language][profile.id]?.summary ?? profile.summary;
  }

  function requirementText(profile: ServiceProfile) {
    const parts = [profile.requiresDomain ? t.domain : t.noDomain];
    if (profile.requiresTLSCert) {
      parts.push(t.tls);
    }
    if (profile.requiresUdp) {
      parts.push(t.udp);
    }
    return parts.join(' + ');
  }
</script>

<svelte:head>
  <title>{t.title} - OMO</title>
  <meta
    name="description"
    content="OMO service library for authorized boundary access and infrastructure operations."
  />
</svelte:head>

{#snippet actions()}
  <button class="icon-button" type="button" aria-label={t.refresh} onclick={loadServices} disabled={loading}>
    <RotateCcw size={17} class={loading ? 'spin' : ''} />
  </button>
{/snippet}

<ConsoleShell
  title={t.title}
  eyebrow={t.phase}
  activeHref="/services"
  statusLabel={coreReady ? t.coreReady : t.coreAttention}
  statusReady={coreReady}
  {actions}
>
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

  <section class="service-section">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">{t.phase}</p>
        <h2>{t.defaultServices}</h2>
      </div>
    </div>

    {#if error}
      <p class="error-text">{error}</p>
    {/if}

    {#if loading}
      <div class="loading-row">
        <LoaderCircle class="spin" size={18} />
        <span>{t.loadingCatalog}</span>
      </div>
    {:else}
      {#if services.length > 0}
        <div class="instance-list" aria-label={t.instances}>
          {#each services as service}
            <article class="instance-row">
              <div>
                <strong>{service.displayName}</strong>
                <span>{service.profileId} / {service.status}</span>
              </div>
              <code>{service.listenPort === 0 ? t.plannedEntry : `:${service.listenPort}`}</code>
            </article>
          {/each}
        </div>
      {:else}
        <p class="empty-text">{t.emptyInstances}</p>
      {/if}

      <div class="service-grid">
        {#each profiles as profile}
          {@const Icon = profileIcon(profile)}
          <article class="service-card">
            <div class="service-title">
              <div class="service-icon">
                <Icon size={20} />
              </div>
              <div>
                <h3>{profileName(profile)}</h3>
                <p>{profileSummary(profile)}</p>
              </div>
            </div>

            <dl class="service-facts">
              <div>
                <dt>{t.transport}</dt>
                <dd>{profile.transport}</dd>
              </div>
              <div>
                <dt>{t.security}</dt>
                <dd>{profile.securityLayer}</dd>
              </div>
              <div>
                <dt>{t.requirements}</dt>
                <dd>{requirementText(profile)}</dd>
              </div>
            </dl>

            <div class="client-list" aria-label={t.clientCompatibility}>
              {#each profile.clientFormats.slice(0, 4) as format}
                <span>{format}</span>
              {/each}
            </div>

            <div class="service-actions">
              <button
                class="secondary-action"
                type="button"
                onclick={() => planProfile(profile)}
                disabled={busyProfile !== ''}
              >
                <Boxes size={16} />
                {t.plan}
              </button>
              <button
                type="button"
                onclick={() => runConfigAction(profile, 'apply')}
                disabled={busyProfile !== ''}
              >
                <ShieldCheck size={16} />
                {t.apply}
              </button>
              <button
                class="secondary-action"
                type="button"
                onclick={() => runConfigAction(profile, 'rollback')}
                disabled={busyProfile !== ''}
              >
                <RotateCcw size={16} />
                {t.rollback}
              </button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</ConsoleShell>
