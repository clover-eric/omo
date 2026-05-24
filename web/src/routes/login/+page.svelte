<script lang="ts">
  import LogIn from '@lucide/svelte/icons/log-in';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import { onMount } from 'svelte';
  import { apiPost } from '$lib/api';

  type Lang = 'zh-CN' | 'en-US';
  let language = $state<Lang>('zh-CN');
  let username = $state('admin');
  let password = $state('');
  let submitting = $state(false);
  let errorMessage = $state('');

  const copy = {
    'zh-CN': {
      title: '登录 - OMO 边界运维',
      subtitle: '边界运维管理平台',
      username: '管理员用户名',
      password: '管理员密码',
      submit: '登录',
      failed: '登录失败，请重试。'
    },
    'en-US': {
      title: 'Login - OMO Boundary Operations',
      subtitle: 'Boundary Operations Platform',
      username: 'Administrator username',
      password: 'Administrator password',
      submit: 'Sign in',
      failed: 'Login failed. Please retry.'
    }
  } as const;

  onMount(() => {
    language = ((localStorage.getItem('omo-language') as Lang | null) ?? 'zh-CN');
    window.addEventListener('omo-language-change', handleLanguageChange);
    return () => window.removeEventListener('omo-language-change', handleLanguageChange);
  });

  function handleLanguageChange(event: Event) {
    const next = (event as CustomEvent<Lang>).detail;
    language = next === 'en-US' ? 'en-US' : 'zh-CN';
  }

  async function login() {
    errorMessage = '';
    submitting = true;
    try {
      await apiPost('/api/auth/login', { username, password });
      window.location.href = '/dashboard';
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : copy[language].failed;
    } finally {
      submitting = false;
    }
  }

  const text = $derived(copy[language]);
</script>

<svelte:head>
  <title>{text.title}</title>
</svelte:head>

<main class="login-page">
  <form class="login-form" onsubmit={(event) => { event.preventDefault(); login(); }}>
    <div class="login-mark">
      <ShieldCheck size={26} />
    </div>
    <h1>OMO</h1>
    <p>{text.subtitle}</p>

    <label>
      <span>{text.username}</span>
      <input bind:value={username} autocomplete="username" required />
    </label>

    <label>
      <span>{text.password}</span>
      <input bind:value={password} type="password" autocomplete="current-password" required />
    </label>

    {#if errorMessage}
      <p class="error-text">{errorMessage}</p>
    {/if}

    <button type="submit" disabled={submitting}>
      {#if submitting}
        <LoaderCircle size={18} class="spin" />
      {:else}
        <LogIn size={18} />
      {/if}
      <span>{text.submit}</span>
    </button>
  </form>
</main>
