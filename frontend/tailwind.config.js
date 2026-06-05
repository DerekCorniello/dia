/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{svelte,ts,js}'],
  theme: {
    extend: {
      colors: {
        bg: {
          900: '#0f172a',
          800: '#1b2636',
          700: '#243144',
          600: '#2f3d52',
        },
        fg: {
          DEFAULT: '#d6deeb',
          dim: '#8b9bb4',
          mute: '#5b6b85',
        },
        accent: {
          DEFAULT: '#7fdbca',
          warn: '#ecc48d',
          err: '#ff5874',
        },
      },
      fontFamily: {
        sans: [
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'sans-serif',
        ],
        mono: [
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Consolas',
          'monospace',
        ],
      },
    },
  },
  plugins: [],
};
