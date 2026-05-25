<script lang="ts">
  import Boxes from '@lucide/svelte/icons/boxes';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import RadioTower from '@lucide/svelte/icons/radio-tower';
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
  import Server from '@lucide/svelte/icons/server';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import Zap from '@lucide/svelte/icons/zap';
  import { onMount } from 'svelte';
  import ConsoleShell from '$lib/ConsoleShell.svelte';
  import { localizedErrorMessage } from '$lib/localizedErrors';
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

  type ProfileCopy = {
    name: string;
    summary: string;
    bestFor: string;
    notFor: string;
    transport: string;
    security: string;
  };

  type Copy = {
    title: string;
    phase: string;
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
    loadError: string;
    createError: string;
    actionError: string;
    loadingCatalog: string;
    workflow: string;
    workflowPlan: string;
    workflowPlanNote: string;
    workflowApply: string;
    workflowApplyNote: string;
    workflowDistribute: string;
    workflowDistributeNote: string;
    recommendedPlan: string;
    recommendedPlanNote: string;
    selectedService: string;
    selectedServiceNote: string;
    status: string;
    bestFor: string;
    notFor: string;
    requirements: string;
    expertDetails: string;
    clientCompatibility: string;
    activeInstances: string;
    noInstances: string;
    port: string;
    plannedEntry: string;
    configVersion: string;
    plan: string;
    apply: string;
    rollback: string;
    openDistribution: string;
    nextPlan: string;
    nextApply: string;
    nextDistribute: string;
    active: string;
    planned: string;
    disabled: string;
    readyForDistribution: string;
    needsApply: string;
    notPlanned: string;
    domain: string;
    noDomain: string;
    tls: string;
    udp: string;
    recommended: string;
  };

  const copy: Record<Language, Copy> = {
    'zh-CN': {
      title: '服务库',
      phase: '第三阶段',
      accessCore: '接入核心',
      ready: '就绪',
      needsAttention: '需要处理',
      loadingCore: '正在读取核心状态',
      profileTemplates: '服务画像',
      backendCatalog: '后端托管的接入方案',
      managedServices: '接入实例',
      managedByBackend: '实例由后端写入并应用',
      operationsMode: '运维模式',
      authorized: '授权',
      authorizedNote: '仅用于边界接入和授权基础设施管理',
      coreReady: '核心就绪',
      coreAttention: '核心需处理',
      refresh: '刷新服务数据',
      loadError: '无法加载服务库数据。',
      createError: '无法创建接入实例。',
      actionError: '无法执行服务配置操作。',
      loadingCatalog: '正在加载服务库...',
      workflow: '核心流程',
      workflowPlan: '选择方案',
      workflowPlanNote: '按业务目标选择接入画像',
      workflowApply: '应用配置',
      workflowApplyNote: '后端生成、验证并写入核心配置',
      workflowDistribute: '分发导入',
      workflowDistributeNote: '到配置分发页生成授权入口',
      recommendedPlan: '推荐接入方案',
      recommendedPlanNote: '普通用户只需要按目标选择服务画像，底层协议细节保留在专家信息中。',
      selectedService: '方案详情',
      selectedServiceNote: '配置始终由后端生成和应用，前端只发起受控操作。',
      status: '状态',
      bestFor: '适合',
      notFor: '不适合',
      requirements: '依赖条件',
      expertDetails: '专家信息',
      clientCompatibility: '客户端兼容',
      activeInstances: '关联实例',
      noInstances: '该方案尚未创建实例。',
      port: '监听端口',
      plannedEntry: '待后端分配',
      configVersion: '配置版本',
      plan: '规划实例',
      apply: '应用配置',
      rollback: '回滚配置',
      openDistribution: '前往配置分发',
      nextPlan: '先规划实例',
      nextApply: '应用后即可分发',
      nextDistribute: '可进入配置分发',
      active: '运行中',
      planned: '已规划',
      disabled: '已停用',
      readyForDistribution: '可分发',
      needsApply: '待应用',
      notPlanned: '未规划',
      domain: '域名',
      noDomain: '无需域名',
      tls: '证书',
      udp: 'UDP',
      recommended: '推荐'
    },
    'en-US': {
      title: 'Service Library',
      phase: 'Phase 3',
      accessCore: 'Access core',
      ready: 'Ready',
      needsAttention: 'Needs attention',
      loadingCore: 'Loading core status',
      profileTemplates: 'Service profiles',
      backendCatalog: 'Backend-owned access plans',
      managedServices: 'Access instances',
      managedByBackend: 'Instances are stored and applied by the backend',
      operationsMode: 'Operations mode',
      authorized: 'Authorized',
      authorizedNote: 'Boundary access and infrastructure management only',
      coreReady: 'Core ready',
      coreAttention: 'Core attention',
      refresh: 'Refresh service data',
      loadError: 'Unable to load service library data.',
      createError: 'Unable to create access instance.',
      actionError: 'Unable to run service configuration action.',
      loadingCatalog: 'Loading service library...',
      workflow: 'Core Workflow',
      workflowPlan: 'Choose plan',
      workflowPlanNote: 'Select an access profile by operating goal',
      workflowApply: 'Apply config',
      workflowApplyNote: 'Backend generates, validates, and writes core config',
      workflowDistribute: 'Distribute import',
      workflowDistributeNote: 'Create authorized entries on Configuration Distribution',
      recommendedPlan: 'Recommended Access Plan',
      recommendedPlanNote: 'Operators choose service profiles by goal; low-level details stay in expert information.',
      selectedService: 'Plan Details',
      selectedServiceNote: 'Configuration is generated and applied by the backend; the frontend only starts controlled actions.',
      status: 'Status',
      bestFor: 'Best for',
      notFor: 'Not for',
      requirements: 'Requirements',
      expertDetails: 'Expert information',
      clientCompatibility: 'Client compatibility',
      activeInstances: 'Linked instances',
      noInstances: 'No instance has been created for this plan yet.',
      port: 'Listen port',
      plannedEntry: 'Assigned by backend',
      configVersion: 'Config version',
      plan: 'Plan instance',
      apply: 'Apply config',
      rollback: 'Rollback config',
      openDistribution: 'Open distribution',
      nextPlan: 'Plan an instance first',
      nextApply: 'Apply before distribution',
      nextDistribute: 'Ready for distribution',
      active: 'Active',
      planned: 'Planned',
      disabled: 'Disabled',
      readyForDistribution: 'Ready to distribute',
      needsApply: 'Needs apply',
      notPlanned: 'Not planned',
      domain: 'Domain',
      noDomain: 'No domain required',
      tls: 'TLS',
      udp: 'UDP',
      recommended: 'Recommended'
    }
  };

  const localizedProfiles: Record<Language, Record<string, ProfileCopy>> = {
    'zh-CN': {
      'standard-secure-access': {
        name: '标准安全接入',
        summary: '面向日常边界运维的默认方案，优先平衡稳定性、可维护性和安全基线。',
        bestFor: '大多数服务器和常规远程运维设备。',
        notFor: '极端高吞吐或弱网移动场景。',
        transport: '经托管 HTTPS 入口承载的 TCP 接入',
        security: '域名证书配合后端生成的访问凭据'
      },
      'high-throughput-access': {
        name: '高吞吐接入',
        summary: '面向大流量业务链路，优先保障吞吐和连接效率。',
        bestFor: 'UDP 可用且网络质量稳定的高带宽场景。',
        notFor: 'UDP 受限、证书未就绪或网络抖动明显的环境。',
        transport: '经托管 UDP 入口承载的 QUIC 接入',
        security: '后端生成凭据，并绑定域名证书材料'
      },
      'broad-compatibility-access': {
        name: '广泛兼容接入',
        summary: '优先覆盖更多客户端和网络环境，适合复杂终端接入。',
        bestFor: '多客户端混用、旧设备和企业网络环境。',
        notFor: '追求最高吞吐的单一高性能链路。',
        transport: '经托管服务入口承载的 TCP 兼容接入',
        security: '后端生成凭据，可在域名就绪后绑定证书'
      },
      'lightweight-fallback-access': {
        name: '轻量备用接入',
        summary: '用于资源有限或临时恢复场景，保持配置简单可回退。',
        bestFor: '低资源服务器、恢复通道和应急保底入口。',
        notFor: '长期承载大规模设备接入。',
        transport: '经后端托管备用入口承载的 TCP 接入',
        security: '后端生成凭据，降低运行时资源开销'
      },
      'mobile-optimized-access': {
        name: '移动优化接入',
        summary: '面向移动设备和不稳定网络，降低切换和恢复成本。',
        bestFor: '移动终端、频繁网络切换和弱网恢复场景。',
        notFor: '固定机房内高吞吐服务链路。',
        transport: '面向重连优化的自适应托管入口',
        security: '后端生成凭据，并在证书可用时绑定域名证书'
      }
    },
    'en-US': {
      'standard-secure-access': {
        name: 'Standard secure access',
        summary: 'Default access plan for daily boundary operations, balancing stability, maintainability, and security.',
        bestFor: 'Most servers and routine remote operations devices.',
        notFor: 'Extreme throughput or unstable mobile-network scenarios.',
        transport: 'TCP access carried by the managed HTTPS entry',
        security: 'Domain certificate with backend-generated access credentials'
      },
      'high-throughput-access': {
        name: 'High throughput access',
        summary: 'Prioritizes throughput and connection efficiency for larger service links.',
        bestFor: 'High-bandwidth environments where UDP is available and stable.',
        notFor: 'UDP-restricted, certificate-missing, or high-jitter environments.',
        transport: 'QUIC access carried by the managed UDP entry',
        security: 'Backend-generated credentials bound to domain certificate material'
      },
      'broad-compatibility-access': {
        name: 'Broad compatibility access',
        summary: 'Prioritizes client coverage and import success across mixed environments.',
        bestFor: 'Mixed clients, older devices, and enterprise network environments.',
        notFor: 'A single link tuned only for maximum throughput.',
        transport: 'TCP-compatible access carried by the managed service entry',
        security: 'Backend-generated credentials with certificate binding after domain readiness'
      },
      'lightweight-fallback-access': {
        name: 'Lightweight fallback access',
        summary: 'Keeps a simple fallback plan for constrained resources or temporary recovery.',
        bestFor: 'Low-resource servers, recovery channels, and emergency fallback access.',
        notFor: 'Long-term high-volume device access.',
        transport: 'TCP access carried by the backend-managed fallback entry',
        security: 'Backend-generated credentials with lower runtime overhead'
      },
      'mobile-optimized-access': {
        name: 'Mobile optimized access',
        summary: 'Reduces reconnection and recovery cost for mobile devices and unstable networks.',
        bestFor: 'Mobile endpoints, frequent network switching, and weak-network recovery.',
        notFor: 'Fixed datacenter links that mainly need high throughput.',
        transport: 'Adaptive managed entry optimized for reconnection',
        security: 'Backend-generated credentials bound to certificates when available'
      }
    }
  };

  let profiles = $state<ServiceProfile[]>([]);
  let services = $state<ServiceInstance[]>([]);
  let coreStatus = $state<SingBoxStatus | null>(null);
  let loading = $state(true);
  let error = $state('');
  let busyProfile = $state('');
  let selectedProfileId = $state('');
  let lastJob = $state<ServiceConfigJobResult | null>(null);

  let t = $derived(copy[$preferences.language]);
  let selectedProfile = $derived(
    profiles.find((profile) => profile.id === selectedProfileId) ?? profiles[0] ?? null
  );
  let coreReady = $derived(Boolean(coreStatus?.installed && coreStatus?.healthy));
  let metrics = $derived([
    { label: t.accessCore, value: coreReady ? t.ready : t.needsAttention, note: coreStatus?.message ?? t.loadingCore, icon: Server },
    { label: t.profileTemplates, value: profiles.length.toString(), note: t.backendCatalog, icon: Boxes },
    { label: t.managedServices, value: services.length.toString(), note: lastJob ? lastJob.job.userMessage : t.managedByBackend, icon: ClipboardList },
    { label: t.operationsMode, value: t.authorized, note: t.authorizedNote, icon: ShieldCheck }
  ]);

  $effect(() => {
    if (profiles.length > 0 && !selectedProfileId) {
      selectedProfileId = profiles[0].id;
    }
  });

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
      profiles = serviceResult.profiles ?? [];
      services = serviceResult.services ?? [];
      coreStatus = statusResult;
    } catch (err) {
      error = localizedErrorMessage(err, $preferences.language, t.loadError);
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
      selectedProfileId = profile.id;
    } catch (err) {
      error = localizedErrorMessage(err, $preferences.language, t.createError);
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
        upsertServices(lastJob.instances ?? []);
      }
      selectedProfileId = profile.id;
    } catch (err) {
      error = localizedErrorMessage(err, $preferences.language, t.actionError);
    } finally {
      busyProfile = '';
    }
  }

  async function runNextAction(profile: ServiceProfile) {
    const state = profileState(profile);
    if (state === 'not-planned') {
      await planProfile(profile);
      return;
    }
    if (state === 'planned') {
      await runConfigAction(profile, 'apply');
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
    if (profile.category === 'mobility') {
      return RadioTower;
    }
    return ShieldCheck;
  }

  function profileName(profile: ServiceProfile) {
    return localizedProfiles[$preferences.language][profile.id]?.name ?? profile.displayName;
  }

  function profileSummary(profile: ServiceProfile) {
    return localizedProfiles[$preferences.language][profile.id]?.summary ?? profile.summary;
  }

  function profileDetail(profile: ServiceProfile, field: keyof ProfileCopy) {
    return localizedProfiles[$preferences.language][profile.id]?.[field] ?? profile[field as keyof ServiceProfile] ?? '';
  }

  function profileInstances(profile: ServiceProfile) {
    return services.filter((service) => service.profileId === profile.id);
  }

  function primaryInstance(profile: ServiceProfile) {
    const related = profileInstances(profile);
    return related.find((service) => service.status === 'active') ?? related.find((service) => service.status === 'planned') ?? related[0] ?? null;
  }

  function profileState(profile: ServiceProfile) {
    const instance = primaryInstance(profile);
    if (!instance) {
      return 'not-planned';
    }
    return instance.status;
  }

  function stateText(profile: ServiceProfile) {
    const state = profileState(profile);
    if (state === 'active') {
      return t.readyForDistribution;
    }
    if (state === 'planned') {
      return t.needsApply;
    }
    if (state === 'disabled') {
      return t.disabled;
    }
    return t.notPlanned;
  }

  function nextText(profile: ServiceProfile) {
    const state = profileState(profile);
    if (state === 'active') {
      return t.nextDistribute;
    }
    if (state === 'planned') {
      return t.nextApply;
    }
    return t.nextPlan;
  }

  function statusText(status: ServiceInstance['status']) {
    return t[status] ?? status;
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

  <section class="workflow-strip" aria-label={t.workflow}>
    <div>
      <Boxes size={18} />
      <strong>{t.workflowPlan}</strong>
      <span>{t.workflowPlanNote}</span>
    </div>
    <div>
      <ShieldCheck size={18} />
      <strong>{t.workflowApply}</strong>
      <span>{t.workflowApplyNote}</span>
    </div>
    <div>
      <ClipboardList size={18} />
      <strong>{t.workflowDistribute}</strong>
      <span>{t.workflowDistributeNote}</span>
    </div>
  </section>

  <section class="service-section">
    {#if error}
      <p class="error-text">{error}</p>
    {/if}

    {#if loading}
      <div class="loading-row">
        <LoaderCircle class="spin" size={18} />
        <span>{t.loadingCatalog}</span>
      </div>
    {:else}
      <div class="service-workbench">
        <section class="panel">
          <div class="panel-heading">
            <div>
              <p class="eyebrow">{t.phase}</p>
              <h2>{t.recommendedPlan}</h2>
              <p class="panel-note">{t.recommendedPlanNote}</p>
            </div>
          </div>

          <div class="plan-list">
            {#each profiles as profile, index}
              {@const Icon = profileIcon(profile)}
              <article class="plan-row" class:active-row={selectedProfile?.id === profile.id}>
                <button
                  class="plan-select"
                  type="button"
                  onclick={() => {
                    selectedProfileId = profile.id;
                  }}
                >
                  <div class="service-icon">
                    <Icon size={20} />
                  </div>
                  <div class="plan-main">
                    <div class="plan-title">
                      <h3>{profileName(profile)}</h3>
                      {#if index === 0}
                        <span>{t.recommended}</span>
                      {/if}
                    </div>
                    <p>{profileSummary(profile)}</p>
                    <div class="plan-meta">
                      <span>{stateText(profile)}</span>
                      <span>{requirementText(profile)}</span>
                      <span>{nextText(profile)}</span>
                    </div>
                  </div>
                </button>
                <button
                  type="button"
                  disabled={busyProfile !== '' || profileState(profile) === 'active'}
                  onclick={(event) => {
                    event.stopPropagation();
                    void runNextAction(profile);
                  }}
                >
                  {#if busyProfile.startsWith(`${profile.id}:`)}
                    <LoaderCircle size={16} class="spin" />
                  {:else if profileState(profile) === 'planned'}
                    <ShieldCheck size={16} />
                  {:else}
                    <Boxes size={16} />
                  {/if}
                  {nextText(profile)}
                </button>
              </article>
            {/each}
          </div>
        </section>

        <section class="panel service-detail">
          {#if selectedProfile}
            <div class="panel-heading">
              <div>
                <p class="eyebrow">{t.selectedService}</p>
                <h2>{profileName(selectedProfile)}</h2>
                <p class="panel-note">{t.selectedServiceNote}</p>
              </div>
              <span class="status-chip">{stateText(selectedProfile)}</span>
            </div>

            <dl class="service-facts">
              <div>
                <dt>{t.bestFor}</dt>
                <dd>{profileDetail(selectedProfile, 'bestFor')}</dd>
              </div>
              <div>
                <dt>{t.notFor}</dt>
                <dd>{profileDetail(selectedProfile, 'notFor')}</dd>
              </div>
              <div>
                <dt>{t.requirements}</dt>
                <dd>{requirementText(selectedProfile)}</dd>
              </div>
            </dl>

            <div class="instance-list detail-instances" aria-label={t.activeInstances}>
              <p class="eyebrow">{t.activeInstances}</p>
              {#if profileInstances(selectedProfile).length === 0}
                <p class="empty-text">{t.noInstances}</p>
              {:else}
                {#each profileInstances(selectedProfile) as service}
                  <article class="instance-row">
                    <div>
                      <strong>{service.displayName}</strong>
                      <span>{statusText(service.status)} / {t.configVersion} {service.configVersion}</span>
                    </div>
                    <code>{service.listenPort === 0 ? t.plannedEntry : `${t.port} :${service.listenPort}`}</code>
                  </article>
                {/each}
              {/if}
            </div>

            <div class="expert-box">
              <p class="eyebrow">{t.expertDetails}</p>
              <dl class="service-facts">
                <div>
                  <dt>Transport</dt>
                  <dd>{profileDetail(selectedProfile, 'transport')}</dd>
                </div>
                <div>
                  <dt>Security</dt>
                  <dd>{profileDetail(selectedProfile, 'security')}</dd>
                </div>
                <div>
                  <dt>Profile</dt>
                  <dd>{selectedProfile.id} / {selectedProfile.version}</dd>
                </div>
              </dl>
              <div class="client-list" aria-label={t.clientCompatibility}>
                {#each selectedProfile.clientFormats as format}
                  <span>{format}</span>
                {/each}
              </div>
            </div>

            <div class="service-actions detail-actions">
              <button
                class="secondary-action"
                type="button"
                onclick={() => planProfile(selectedProfile)}
                disabled={busyProfile !== ''}
              >
                <Boxes size={16} />
                {t.plan}
              </button>
              <button
                type="button"
                onclick={() => runConfigAction(selectedProfile, 'apply')}
                disabled={busyProfile !== ''}
              >
                <ShieldCheck size={16} />
                {t.apply}
              </button>
              <button
                class="secondary-action"
                type="button"
                onclick={() => runConfigAction(selectedProfile, 'rollback')}
                disabled={busyProfile !== ''}
              >
                <RotateCcw size={16} />
                {t.rollback}
              </button>
              <a class="primary-action" href="/subscriptions">
                <ClipboardList size={16} />
                {t.openDistribution}
              </a>
            </div>
          {/if}
        </section>
      </div>
    {/if}
  </section>
</ConsoleShell>
