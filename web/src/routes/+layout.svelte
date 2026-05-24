<script lang="ts">
  import Languages from '@lucide/svelte/icons/languages';
  import Moon from '@lucide/svelte/icons/moon';
  import Sun from '@lucide/svelte/icons/sun';
  import { onMount } from 'svelte';
  import '../styles.css';

  type Lang = 'zh-CN' | 'en-US';
  type Theme = 'light' | 'dark';

  let { children } = $props();
  let language = $state<Lang>('zh-CN');
  let theme = $state<Theme>('light');

  onMount(() => {
    language = readLanguage();
    theme = readTheme();
    applyPreferences();
  });

  function readLanguage(): Lang {
    return localStorage.getItem('omo-language') === 'en-US' ? 'en-US' : 'zh-CN';
  }

  function readTheme(): Theme {
    return localStorage.getItem('omo-theme') === 'dark' ? 'dark' : 'light';
  }

  function toggleLanguage() {
    language = language === 'zh-CN' ? 'en-US' : 'zh-CN';
    localStorage.setItem('omo-language', language);
    applyPreferences();
    window.dispatchEvent(new CustomEvent<Lang>('omo-language-change', { detail: language }));
  }

  function toggleTheme() {
    theme = theme === 'light' ? 'dark' : 'light';
    localStorage.setItem('omo-theme', theme);
    applyPreferences();
  }

  function applyPreferences() {
    document.documentElement.lang = language;
    document.documentElement.dataset.theme = theme;
  }
</script>

<div class="preference-controls" aria-label={language === 'zh-CN' ? '界面偏好' : 'Interface preferences'}>
  <button
    type="button"
    title={language === 'zh-CN' ? '切换为英文' : 'Switch to Chinese'}
    aria-label={language === 'zh-CN' ? '切换为英文' : 'Switch to Chinese'}
    onclick={toggleLanguage}
  >
    <Languages size={16} />
    <span>{language === 'zh-CN' ? '中' : 'EN'}</span>
  </button>
  <button
    type="button"
    title={theme === 'light' ? (language === 'zh-CN' ? '切换为深色' : 'Switch to dark') : (language === 'zh-CN' ? '切换为浅色' : 'Switch to light')}
    aria-label={theme === 'light' ? (language === 'zh-CN' ? '切换为深色' : 'Switch to dark') : (language === 'zh-CN' ? '切换为浅色' : 'Switch to light')}
    onclick={toggleTheme}
  >
    {#if theme === 'light'}
      <Sun size={16} />
    {:else}
      <Moon size={16} />
    {/if}
  </button>
</div>

{@render children()}
