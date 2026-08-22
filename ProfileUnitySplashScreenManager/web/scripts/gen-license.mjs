/**
 * Writes src/app/primeui-license.generated.ts from the PRIMEUI_LICENSE_KEY
 * environment variable.
 *
 * PrimeNG 18 and later are under the PrimeUI commercial license and take the key
 * as a client-side config value (`providePrimeNG({ license })`), which means it is
 * compiled into the JavaScript bundle and therefore into the shipped executable.
 * That is PrimeTek's intended mechanism for distributing an application, but it
 * does mean a per-developer key becomes extractable from any copy of the build.
 *
 * The key is therefore never committed and never defaulted. It is supplied at
 * build time or not at all:
 *
 *   PRIMEUI_LICENSE_KEY=... npm run build     -> key is embedded
 *   npm run build                             -> no key; PrimeNG shows its
 *                                                "Invalid PrimeUI License" banner
 *
 * The generated file is git-ignored. See README.md, "PrimeNG licensing".
 */
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const target = join(here, '..', 'src', 'app', 'primeui-license.generated.ts');

const key = (process.env.PRIMEUI_LICENSE_KEY ?? '').trim();

const banner = `// GENERATED FILE -- DO NOT EDIT, DO NOT COMMIT.
// Written by scripts/gen-license.mjs from the PRIMEUI_LICENSE_KEY environment
// variable at build time. Git-ignored on purpose: this value is a commercial
// per-developer license key.
`;

mkdirSync(dirname(target), { recursive: true });
writeFileSync(target, `${banner}export const PRIMEUI_LICENSE_KEY = ${JSON.stringify(key)};\n`, 'utf8');

if (key) {
  // Never print the key itself.
  console.log(`gen-license: PRIMEUI_LICENSE_KEY set (${key.length} characters) -- it will be embedded in the bundle.`);
} else {
  console.warn(
    'gen-license: PRIMEUI_LICENSE_KEY is not set.\n' +
    '             The build will succeed, but PrimeNG will display an "Invalid PrimeUI License"\n' +
    '             banner in the running application. Set the variable to embed the key.',
  );
}
