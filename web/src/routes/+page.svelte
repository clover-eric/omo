<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import Boxes from '@lucide/svelte/icons/boxes';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import ClipboardList from '@lucide/svelte/icons/clipboard-list';
  import Gauge from '@lucide/svelte/icons/gauge';
  import Network from '@lucide/svelte/icons/network';
  import RotateCcw from '@lucide/svelte/icons/rotate-ccw';
  import Server from '@lucide/svelte/icons/server';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal';
  import Zap from '@lucide/svelte/icons/zap';
  import { onMount } from 'svelte';
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

  const navItems = [
    { label: 'Overview', icon: Gauge, href: '/dashboard', active: false },
    { label: 'Service Library', icon: Boxes, href: '/services', active: true },
    { label: 'Distribution', icon: ClipboardList, href: '/subscriptions', active: false },
    { label: 'Cascade Nodes', icon: Network, href: '/cascade', active: false },
    { label: 'Server Checkup', icon: Activity, href: '/diagnostics', active: false },
    { label: 'Audit Logs', icon: ClipboardList, href: '/logs', active: false },
    { label: 'Settings', icon: SlidersHorizontal, href: '/settings', active: false }
  ];

  let profiles: ServiceProfile[] = [];
  let services: ServiceInstance[] = [];
  let coreStatus: SingBoxStatus | null = null;
  let loading = true;
  let error = '';
  let busyProfile = '';
  let lastJob: ServiceConfigJobResult | null = null;

  $: coreReady = Boolean(coreStatus?.installed && coreStatus?.healthy);
  $: metrics = [
    { label: 'Access core', value: coreReady ? 'Ready' : 'Needs attention', note: coreStatus?.message ?? 'Loading core status' },
    { label: 'Profile templates', value: profiles.length.toString(), note: 'Backend-owned service catalog' },
    { label: 'Managed services', value: services.length.toString(), note: lastJob ? lastJob.job.userMessage : 'Planned instances are stored by the backend' },
    { label: 'Operations mode', value: 'Authorized', note: 'Boundary access and infrastructure management only' }
  ];

  onMount(() => {
    void loadDashboard();
  });

  async function loadDashboard() {
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
      error = err instanceof Error ? err.message : 'Unable to load dashboard data.';
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
        displayName: profile.displayName,
        listenPort: 0,
        status: 'planned'
      });
      services = [result.service, ...services];
    } catch (err) {
      error = err instanceof Error ? err.message : 'Unable to create managed service.';
    } finally {
      busyProfile = '';
    }
  }

  async function applyProfile(profile: ServiceProfile) {
    await runConfigAction(profile.id, 'apply');
  }

  async function rollbackProfile(profile: ServiceProfile) {
    await runConfigAction(profile.id, 'rollback');
  }

  async function runConfigAction(profileId: string, action: 'apply' | 'rollback') {
    busyProfile = `${profileId}:${action}`;
    error = '';
    try {
      lastJob = await apiPost<ServiceConfigJobResult>(`/api/services/${profileId}/${action}`, {});
      if (lastJob.instances && lastJob.instances.length > 0) {
        upsertServices(lastJob.instances);
      }
    } catch (err) {
      error = err instanceof Error ? err.message : `Unable to ${action} service configuration.`;
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
</script>

<svelte:head>
  <title>OMO Boundary Operations Console</title>
  <meta
    name="description"
    content="OMO boundary operations console for authorized infrastructure management."
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
      {#each navItems as item}
        <a class:active={item.active} href={item.href}>
          <item.icon size={18} strokeWidth={1.8} />
          <span>{item.label}</span>
        </a>
      {/each}
    </nav>
  </aside>

  <main class="workspace">
    <header class="topbar">
      <div>
        <p class="eyebrow">Operations Console</p>
        <h1>Service Library</h1>
      </div>
      <div class:ready={coreReady} class="status-pill">
        <span></span>
        {coreReady ? 'Core ready' : 'Core attention'}
      </div>
    </header>

    <section class="summary-grid" aria-label="Operational summary">
      {#each metrics as metric, index}
        <article class="metric-card">
          <div class="metric-icon">
            {#if index === 0}
              <Server size={20} />
            {:else if index === 1}
              <Boxes size={20} />
            {:else if index === 2}
              <ClipboardList size={20} />
            {:else}
              <ShieldCheck size={20} />
            {/if}
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
          <p class="eyebrow">Phase 3</p>
          <h2>Default Access Services</h2>
        </div>
        <button class="icon-button" type="button" aria-label="Refresh service data" on:click={loadDashboard} disabled={loading}>
          <RotateCcw size={17} />
        </button>
      </div>

      {#if error}
        <p class="error-text">{error}</p>
      {/if}

      {#if loading}
        <div class="loading-row">
          <RotateCcw class="spin" size={18} />
          <span>Loading service catalog...</span>
        </div>
      {:else}
        {#if services.length > 0}
          <div class="instance-list" aria-label="Managed service instances">
            {#each services as service}
              <article class="instance-row">
                <div>
                  <strong>{service.displayName}</strong>
                  <span>{service.profileId} / {service.status}</span>
                </div>
                <code>{service.listenPort === 0 ? 'managed entry' : `:${service.listenPort}`}</code>
              </article>
            {/each}
          </div>
        {/if}

        <div class="service-grid">
          {#each profiles as profile}
            <article class="service-card">
              <div class="service-title">
                <div class="service-icon">
                  <svelte:component this={profileIcon(profile)} size={20} />
                </div>
                <div>
                  <h3>{profile.displayName}</h3>
                  <p>{profile.summary}</p>
                </div>
              </div>

              <dl class="service-facts">
                <div>
                  <dt>Transport</dt>
                  <dd>{profile.transport}</dd>
                </div>
                <div>
                  <dt>Security</dt>
                  <dd>{profile.securityLayer}</dd>
                </div>
                <div>
                  <dt>Requirements</dt>
                  <dd>
                    {profile.requiresDomain ? 'Domain' : 'No domain required'}
                    {profile.requiresTLSCert ? ' + TLS' : ''}
                    {profile.requiresUdp ? ' + UDP' : ''}
                  </dd>
                </div>
              </dl>

              <div class="client-list" aria-label="Client compatibility">
                {#each profile.clientFormats.slice(0, 4) as format}
                  <span>{format}</span>
                {/each}
              </div>

              <div class="service-actions">
                <button
                  class="secondary-action"
                  type="button"
                  on:click={() => planProfile(profile)}
                  disabled={busyProfile !== ''}
                >
                  <Boxes size={16} />
                  Plan
                </button>
                <button
                  type="button"
                  on:click={() => applyProfile(profile)}
                  disabled={busyProfile !== ''}
                >
                  <ShieldCheck size={16} />
                  Apply
                </button>
                <button
                  class="secondary-action"
                  type="button"
                  on:click={() => rollbackProfile(profile)}
                  disabled={busyProfile !== ''}
                >
                  <RotateCcw size={16} />
                  Rollback
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>
  </main>
</div>
