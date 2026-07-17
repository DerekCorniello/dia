import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import svelteConfig from './svelte.config.js';

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs.recommended,
  {
    languageOptions: {
      globals: { ...globals.browser, ...globals.node },
    },
    rules: {
      // Warn, not error: the remaining `any`s all sit on plugin-supplied
      // JSON, where the honest type is `unknown` plus narrowing. That is
      // a typing pass of its own, so it is tracked rather than silenced.
      '@typescript-eslint/no-explicit-any': 'warn',
    },
  },
  {
    files: ['**/*.svelte', '**/*.svelte.ts'],
    languageOptions: {
      parserOptions: {
        projectService: true,
        extraFileExtensions: ['.svelte'],
        parser: ts.parser,
        svelteConfig,
      },
    },
    rules: {
      // typescript-eslint's unused-vars walks the compiler-generated
      // Svelte AST and crashes on it. svelte-check already reports
      // unused values in components.
      '@typescript-eslint/no-unused-vars': 'off',
    },
  },
  {
    // Build configs load plugins with require(); that is the format
    // tailwind and postcss expect.
    files: ['*.config.js', '*.config.ts'],
    rules: {
      '@typescript-eslint/no-require-imports': 'off',
    },
  },
  {
    // wailsjs/ is generated on every wails build; dist/ is output.
    ignores: ['dist/', 'wailsjs/', 'node_modules/'],
  },
);
