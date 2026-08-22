import { ApplicationConfig, provideBrowserGlobalErrorListeners } from '@angular/core';
import { providePrimeNG } from 'primeng/config';
// No Angular animations provider: PrimeNG 22 drives its own transitions through
// @primeuix/motion, so @angular/animations is not a dependency of this build.
import Aura from '@primeuix/themes/aura';
import { definePreset } from '@primeuix/themes';

import { PRIMEUI_LICENSE_KEY } from './primeui-license.generated';

/**
 * Aura, re-pointed at the Liquidware palette.
 *
 * The primary ramp is the style guide's brand blue: 500 is the base brand blue,
 * 600 the primary action colour, 700 hover and 800 active. The surface ramp is
 * the guide's zinc neutrals. Values are the guide's own, from
 * colors_and_type.css.
 */
const LiquidwarePreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: '#f2f8fc',
      100: '#d9ecf7',
      200: '#b3d8ee',
      300: '#7dbde2',
      400: '#3f9bd2',
      500: '#0072bc',
      600: '#0061a0',
      700: '#005084',
      800: '#003f67',
      900: '#002e4b',
      950: '#001d2f',
    },
    colorScheme: {
      light: {
        primary: {
          color: '#0061a0',
          contrastColor: '#ffffff',
          hoverColor: '#005084',
          activeColor: '#003f67',
        },
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
      },
    },
  },
  components: {
    // The guide is explicit that button labels are 400, not bold, and that
    // buttons and fields use the small radius while cards use the large one.
    button: {
      root: {
        borderRadius: '{border.radius.sm}',
        // The style guide is explicit: "button labels are 400, not bold".
        // Aura ships 500.
        label: { fontWeight: '400' },
      },
    },
  },
});

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    providePrimeNG({
      theme: {
        preset: LiquidwarePreset,
        options: {
          darkModeSelector: false,
          // PrimeNG's generated CSS goes in its own layer so Tailwind utilities
          // win without !important. Order matches styles.css.
          cssLayer: { name: 'primeng', order: 'theme, base, primeng' },
        },
      },
      ripple: false,
      // Compiled into the bundle -- see scripts/gen-license.mjs for why, and
      // README.md under "PrimeNG licensing".
      license: PRIMEUI_LICENSE_KEY,
    }),
  ],
};
