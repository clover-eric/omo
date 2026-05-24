<script lang="ts">
  import Languages from '@lucide/svelte/icons/languages';
  import Moon from '@lucide/svelte/icons/moon';
  import Sun from '@lucide/svelte/icons/sun';
  import { onMount } from 'svelte';
  import '../styles.css';

  let { children } = $props();
  let language = $state('zh-CN');
  let theme = $state('light');

  onMount(() => {
    language = localStorage.getItem('omo-language') ?? 'zh-CN';
    theme = localStorage.getItem('omo-theme') ?? 'light';
    applyPreferences();
  });

  function toggleLanguage() {
    language = language === 'zh-CN' ? 'en-US' : 'zh-CN';
    localStorage.setItem('omo-language', language);
    applyPreferences();
    window.dispatchEvent(new CustomEvent('omo-language-change', { detail: language }));
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

<div class="preference-controls" aria-label="界面偏好">
  <button type="button" title="切换中文/English" aria-label="切换中文和英文" onclick={toggleLanguage}>
    <Languages size={16} />
    <span>{language === 'zh-CN' ? '中' : 'EN'}</span>
  </button>
  <button type="button" title="切换黑白主题" aria-label="切换黑白主题" onclick={toggleTheme}>
    {#if theme === 'light'}
      <Sun size={16} />
    {:else}
      <Moon size={16} />
    {/if}
  </button>
</div>

{@render children()}
