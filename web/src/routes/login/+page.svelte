<script lang="ts">
  import LogIn from '@lucide/svelte/icons/log-in';
  import LoaderCircle from '@lucide/svelte/icons/loader-circle';
  import ShieldCheck from '@lucide/svelte/icons/shield-check';
  import { apiPost } from '$lib/api';

  let username = $state('admin');
  let password = $state('');
  let submitting = $state(false);
  let errorMessage = $state('');

  async function login() {
    errorMessage = '';
    submitting = true;
    try {
      await apiPost('/api/auth/login', { username, password });
      window.location.href = '/dashboard';
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Login failed. Please retry.';
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Login - OMO Boundary Operations</title>
</svelte:head>

<main class="login-page">
  <form class="login-form" onsubmit={(event) => { event.preventDefault(); login(); }}>
    <div class="login-mark">
      <ShieldCheck size={26} />
    </div>
    <h1>OMO</h1>
    <p>Boundary Operations Platform</p>

    <label>
      <span>Administrator username</span>
      <input bind:value={username} autocomplete="username" required />
    </label>

    <label>
      <span>Administrator password</span>
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
      <span>Sign in</span>
    </button>
  </form>
</main>
