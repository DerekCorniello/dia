import './style.css';
import { mount } from 'svelte';
import App from './App.svelte';

const target = document.getElementById('app');
if (!target) {
  throw new Error('dia: #app element not found in index.html');
}

// Svelte 5 mounts through mount(), not `new App({ target })`. The old
// form does not establish the component context that legacy lifecycle
// (onMount, and the $effect it compiles to) needs, so it threw
// effect_orphan during App's initialisation and the window came up
// empty. Component *syntax* here is still the legacy dialect, which
// Svelte 5 supports; only the instantiation API changed.
const app = mount(App, { target });

export default app;
