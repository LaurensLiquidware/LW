import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { App } from './app/app';

bootstrapApplication(App, appConfig).catch((err) => {
  // Nothing external is reachable, so surface the failure in the page itself
  // rather than leaving a blank window.
  console.error(err);
  const target = document.body;
  if (target) {
    const pre = document.createElement('pre');
    pre.style.cssText = 'padding:16px;font:13px/1.5 monospace;color:#dc2626;white-space:pre-wrap';
    pre.textContent = `The user interface failed to start.\n\n${err instanceof Error ? err.stack ?? err.message : String(err)}`;
    target.appendChild(pre);
  }
});
