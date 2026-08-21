/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{html,ts}'],
  darkMode: ['class', '.dark'],
  corePlugins: {
    // PrimeNG (via the Aura preset) and the Liquidware design tokens own
    // base element styling; Tailwind is used utility-first for layout and
    // spacing only. Preflight's reset would otherwise fight PrimeNG's own
    // component base styles.
    preflight: false,
  },
  theme: {
    extend: {},
  },
  plugins: [],
};
