<script lang="ts">
  import logo from './assets/images/logo-universal.png'
</script>

<main>
  <img alt="dia" id="logo" src="{logo}" />
  <div class="title">dia</div>
  <div class="subtitle">dev workspace launcher</div>
  <div class="hint">Phase 0 skeleton. Real UI lands in Phase 5.</div>
</main>

<style>
  main {
    text-align: center;
    padding: 4rem 2rem;
    color: #d6deeb;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  }
  #logo {
    display: block;
    width: 120px;
    height: 120px;
    margin: 0 auto 2rem;
  }
  .title {
    font-size: 3rem;
    font-weight: 600;
    margin-bottom: 0.5rem;
  }
  .subtitle {
    font-size: 1.25rem;
    color: #7f8fa4;
    margin-bottom: 3rem;
  }
  .hint {
    font-size: 0.875rem;
    color: #4a5666;
    font-style: italic;
  }
</style>
