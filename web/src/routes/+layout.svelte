<script lang="ts">
  import Languages from '@lucide/svelte/icons/languages';
  import Moon from '@lucide/svelte/icons/moon';
  import Sun from '@lucide/svelte/icons/sun';
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { consoleLabels } from '$lib/consoleCopy';
  import { initPreferences, preferences, toggleLanguage, toggleTheme } from '$lib/preferences';
  import '../styles.css';

  let { children } = $props();
  let labels = $derived(consoleLabels($preferences.language));
  let showFloatingPreferences = $derived(!isConsolePath(page.url.pathname));

  onMount(() => {
    initPreferences();
  });

  function isConsolePath(path: string) {
    return [
      '/',
      '/dashboard',
      '/services',
      '/subscriptions',
      '/cascade',
      '/diagnostics',
      '/logs',
      '/settings'
    ].includes(path);
  }
</script>

{#if showFloatingPreferences}
  <div class="preference-controls" aria-label={labels.interfacePreferences}>
    <button
      type="button"
      title={$preferences.language === 'zh-CN' ? labels.switchToEnglish : labels.switchToChinese}
      aria-label={$preferences.language === 'zh-CN' ? labels.switchToEnglish : labels.switchToChinese}
      onclick={toggleLanguage}
    >
      <Languages size={16} />
      <span>{labels.currentLanguageShort}</span>
    </button>
    <button
      type="button"
      title={$preferences.theme === 'light' ? labels.switchToDark : labels.switchToLight}
      aria-label={$preferences.theme === 'light' ? labels.switchToDark : labels.switchToLight}
      onclick={toggleTheme}
    >
      {#if $preferences.theme === 'light'}
        <Sun size={16} />
      {:else}
        <Moon size={16} />
      {/if}
    </button>
  </div>
{/if}

{@render children()}
