// focusTrap is a Svelte action that keeps Tab navigation inside a modal
// dialog and restores focus to the previously focused element on
// destroy. Use on every modal root: use:focusTrap.
export function focusTrap(node: HTMLElement) {
  const previous = document.activeElement as HTMLElement | null;
  const selector =
    'a[href], button:not([disabled]), textarea, input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

  function onKey(e: KeyboardEvent) {
    if (e.key !== 'Tab') return;
    const items = Array.from(node.querySelectorAll<HTMLElement>(selector)).filter(
      (el) => el.offsetParent !== null,
    );
    if (items.length === 0) {
      e.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  node.addEventListener('keydown', onKey);
  // Focus the first control on mount so keyboard users land inside.
  const first = node.querySelector<HTMLElement>(selector);
  (first ?? node).focus?.();

  return {
    destroy() {
      node.removeEventListener('keydown', onKey);
      previous?.focus?.();
    },
  };
}
