import { definePreset } from '@primeng/themes';
import Aura from '@primeng/themes/aura';

/**
 * PrimeNG Aura preset re-themed with the Liquidware design tokens
 * (docs/design-system-reference/liquidware-ui/colors_and_type.css) — the
 * project brief §8 requires using these values as given, never
 * approximated or invented.
 *
 * Aura's default light scheme uses a "slate" surface palette; ours uses
 * the same zinc-based scale in both light and dark (see --p-surface-* in
 * tokens.css), so only `light.surface` needs overriding here — Aura's
 * dark surface palette is already zinc.
 */
export const LiquidwarePreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: '#f2f8fc',
      100: '#c2ddef',
      200: '#91c2e2',
      300: '#61a8d5',
      400: '#308dc9',
      500: '#0072bc',
      600: '#0061a0',
      700: '#005084',
      800: '#003f67',
      900: '#002e4b',
      950: '#001d2f',
    },
    colorScheme: {
      light: {
        surface: {
          0: '#ffffff',
          50: '#fafafa',
          100: '#f4f4f5',
          200: '#e4e4e7',
          300: '#d4d4d8',
          400: '#a1a1aa',
          500: '#71717a',
          600: '#52525b',
          700: '#3f3f46',
          800: '#27272a',
          900: '#18181b',
          950: '#09090b',
        },
        primary: {
          color: '{primary.600}',
          contrastColor: '#ffffff',
          hoverColor: '{primary.700}',
          activeColor: '{primary.800}',
        },
      },
      dark: {
        primary: {
          color: '{primary.500}',
          contrastColor: '#ffffff',
          hoverColor: '{primary.400}',
          activeColor: '{primary.300}',
        },
      },
    },
  },
});
