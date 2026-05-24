<script lang="ts">
  import Activity from '@lucide/svelte/icons/activity';
  import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
  import KeyRound from '@lucide/svelte/icons/key-round';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import { onMount } from 'svelte';
  import { apiGet, apiPost, type BootstrapEvent, type BootstrapJob, type BootstrapStatus } from '$lib/api';
  import { formatBootstrapState, type BootstrapState } from '$lib/status';

  type Lang = 'zh-CN' | 'en-US';

  let language = $state<Lang>('zh-CN');
  let token = $state('');
  let username = $state('admin');
  let password = $state('');
  let confirmPassword = $state('');
  let domain = $state('');
  let loading = $state(true);
  let submitting = $state(false);
  let status = $state<BootstrapStatus | null>(null);
  let latestJob = $state<BootstrapJob | null>(null);
  let events = $state<BootstrapEvent[]>([]);
  let errorMessage = $state('');

  const copy = {
    'zh-CN': {
      title: '初始化 - OMO 边界运维',
      product: 'OMO 边界运维管理平台',
      heading: '初始化',
      progress: '初始化进度',
      formTitle: '管理员与域名',
      loading: '正在读取初始化状态...',
      token: '一次性初始化令牌',
      username: '管理员用户名',
      password: '管理员密码',
      confirmPassword: '确认密码',
      domain: '解析到本服务器的域名',
      start: '开始配置部署',
      retryStart: '确认重试并继续部署',
      complete: '入口配置完成',
      retry: '重试初始化',
      liveSteps: '实时步骤',
      eventLog: '初始化日志',
      waiting: '等待初始化事件',
      eventError: '初始化事件暂不可用，请刷新后重试。',
      requestError: '请求失败，请重试。'
    },
    'en-US': {
      title: 'Initialization - OMO Boundary Operations',
      product: 'OMO Boundary Operations Platform',
      heading: 'Initialization',
      progress: 'Initialization progress',
      formTitle: 'Administrator And Domain',
      loading: 'Loading initialization status...',
      token: 'One-time initialization token',
      username: 'Administrator username',
      password: 'Administrator password',
      confirmPassword: 'Confirm password',
      domain: 'Domain resolving to this server',
      start: 'Start automated configuration',
      retryStart: 'Confirm retry and continue',
      complete: 'Entry configuration complete',
      retry: 'Retry initialization',
      liveSteps: 'Live steps',
      eventLog: 'Initialization log',
      waiting: 'Waiting for initialization events',
      eventError: 'Initialization events are unavailable. Refresh and retry.',
      requestError: 'Request failed. Please retry.'
    }
  } as const;

  const steps = [
    'PREFLIGHT_CHECK',
    'ADMIN_CREATE',
    'DOMAIN_VERIFY',
    'TLS_PROVISION',
    'PANEL_HTTPS_ENABLE',
    'CORE_INSTALL',
    'FINAL_HEALTH_CHECK'
  ] as BootstrapState[];

  onMount(() => {
    language = readLanguage();
    window.addEventListener('omo-language-change', handleLanguageChange);
    void initialize();
    return () => window.removeEventListener('omo-language-change', handleLanguageChange);
  });

  function readLanguage(): Lang {
    return localStorage.getItem('omo-language') === 'en-US' ? 'en-US' : 'zh-CN';
  }

  async function initialize() {
    token = new URLSearchParams(window.location.search).get('token') ?? '';
    await loadStatus();
    loading = false;
    connectEvents();
  }

  function handleLanguageChange(event: Event) {
    const next = (event as CustomEvent<Lang>).detail;
    language = next === 'en-US' ? 'en-US' : 'zh-CN';
  }

  async function loadStatus() {
    try {
      status = await apiGet<BootstrapStatus>('/api/bootstrap/status');
      latestJob = status.latestJob ?? null;
      if (status.domain) {
        domain = status.domain;
      }
    } catch (error) {
      errorMessage = messageFrom(error);
    }
  }

  async function startBootstrap(retry = latestJob?.status === 'failed') {
    errorMessage = '';
    submitting = true;
    try {
      const result = await apiPost<{ job: BootstrapJob; redirectTo: string }>('/api/bootstrap/start', {
        token,
        username,
        password,
        confirmPassword,
        domain,
        retry
      });
      latestJob = result.job;
      await loadStatus();
      if (result.redirectTo) {
        window.setTimeout(() => {
          window.location.href = result.redirectTo;
        }, 3000);
      }
    } catch (error) {
      const message = messageFrom(error);
      if (!retry && message.includes('previous initialization attempt failed')) {
        await startBootstrap(true);
        return;
      }
      errorMessage = message;
    } finally {
      submitting = false;
    }
  }

  function connectEvents() {
    const source = new EventSource('/api/bootstrap/events');
    source.addEventListener('bootstrap', (event) => {
      const item = JSON.parse((event as MessageEvent).data) as BootstrapEvent;
      events = [...events, item].slice(-80);
      latestJob = {
        id: item.jobId,
        kind: item.kind,
        state: item.state,
        status: item.status,
        progress: item.progress,
        userMessage: item.message
      };
    });
    source.addEventListener('error', () => {
      if (events.length === 0) {
        errorMessage = copy[language].eventError;
      }
    });
  }

  function messageFrom(error: unknown) {
    return error instanceof Error ? error.message : copy[language].requestError;
  }

  function eventMessage(message: string) {
    if (language === 'zh-CN') {
      return message;
    }
    const translated: Record<string, string> = {
      '初始化任务已创建。': 'Initialization job created.',
      '正在执行初始化预检查。': 'Running initialization preflight checks.',
      '初始化预检查通过。': 'Initialization preflight checks passed.',
      '正在创建管理员账户。': 'Creating administrator account.',
      '管理员账户已创建。': 'Administrator account created.',
      '检测到可重试的初始化状态。': 'Retryable initialization state detected.',
      '管理员账户已存在，继续入口配置。': 'Administrator account exists; continuing entry configuration.',
      '正在校验域名解析和入口端口。': 'Verifying domain resolution and entry ports.',
      '域名解析和端口检查通过。': 'Domain resolution and port checks passed.',
      '证书状态已记录。': 'Certificate status recorded.',
      '控制台 HTTPS 入口配置已应用。': 'Console HTTPS entry configuration applied.',
      '初始化入口配置完成，正在切换到正式控制台服务。': 'Initialization entry configuration complete; switching to the regular console service.'
    };
    return translated[message] ?? message;
  }

  const text = $derived(copy[language]);
  const currentProgress = $derived(latestJob?.progress ?? 0);
  const complete = $derived(status?.phase1Complete || latestJob?.status === 'succeeded');
  const retrying = $derived(latestJob?.status === 'failed');
</script>

<svelte:head>
  <title>{text.title}</title>
</svelte:head>

<main class="init-page">
  <section class="init-hero">
    <div class="init-heading">
      <div class="init-mark">
        <ShieldCheck size={24} />
      </div>
      <div>
        <p>{text.product}</p>
        <h1>{text.heading}</h1>
      </div>
    </div>
    <div class="progress-ring" aria-label={text.progress}>
      <span>{currentProgress}%</span>
    </div>
  </section>

  <section class="init-grid">
    <form class="init-form" onsubmit={(event) => { event.preventDefault(); startBootstrap(); }}>
      <div class="form-title">
        <KeyRound size={20} />
        <h2>{text.formTitle}</h2>
      </div>

      {#if loading}
        <div class="loading-row">
          <LoaderCircle size={18} class="spin" />
          <span>{text.loading}</span>
        </div>
      {:else}
        <label>
          <span>{text.token}</span>
          <input bind:value={token} autocomplete="one-time-code" required />
        </label>

        <label>
          <span>{text.username}</span>
          <input bind:value={username} autocomplete="username" required minlength="3" maxlength="64" />
        </label>

        <label>
          <span>{text.password}</span>
          <input bind:value={password} type="password" autocomplete="new-password" required minlength="8" />
        </label>

        <label>
          <span>{text.confirmPassword}</span>
          <input bind:value={confirmPassword} type="password" autocomplete="new-password" required minlength="8" />
        </label>

        <label>
          <span>{text.domain}</span>
          <input bind:value={domain} inputmode="url" placeholder="ops.example.com" required />
        </label>

        {#if errorMessage}
          <p class="error-text">{errorMessage}</p>
        {/if}

        <button type="submit" disabled={submitting || complete}>
          {#if submitting}
            <LoaderCircle size={18} class="spin" />
          {:else if complete}
            <CheckCircle2 size={18} />
          {:else}
            <Activity size={18} />
          {/if}
          <span>{complete ? text.complete : retrying ? text.retryStart : text.start}</span>
        </button>
      {/if}
    </form>

    <aside class="init-status">
      <div class="status-header">
        <p>{text.liveSteps}</p>
        <strong>{formatBootstrapState((latestJob?.state ?? status?.state ?? 'UNINITIALIZED') as BootstrapState, language)}</strong>
      </div>

      <div class="bar">
        <span style={`width: ${currentProgress}%`}></span>
      </div>

      <div class="step-list">
        {#each steps as step}
          <div class:done={currentProgress >= ((steps.indexOf(step) + 1) / steps.length) * 100}>
            <span></span>
            <p>{formatBootstrapState(step, language)}</p>
          </div>
        {/each}
      </div>

      <div class="event-log" aria-label={text.eventLog}>
        {#if events.length === 0}
          <p class="muted">{text.waiting}</p>
        {:else}
          {#each events as event}
            <article>
              <span>{event.progress}%</span>
              <p>{eventMessage(event.message)}</p>
            </article>
          {/each}
        {/if}
      </div>
    </aside>
  </section>
</main>
