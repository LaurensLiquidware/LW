import { Component, computed, inject, signal, OnDestroy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DatePipe } from '@angular/common';

import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { TableModule } from 'primeng/table';
import { DialogModule } from 'primeng/dialog';
import { MessageModule } from 'primeng/message';
import { TooltipModule } from 'primeng/tooltip';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { ConfirmationService } from 'primeng/api';

import { ApiService, ApiError } from './api.service';
import { AboutInfo, AppState, HistoryEntry } from './models';

/** A status line under the action row. */
interface Status {
  text: string;
  kind: 'good' | 'poor' | 'fair';
}

@Component({
  selector: 'psm-root',
  standalone: true,
  imports: [
    FormsModule,
    DatePipe,
    ButtonModule,
    InputTextModule,
    TableModule,
    DialogModule,
    MessageModule,
    TooltipModule,
    ConfirmDialogModule,
  ],
  providers: [ConfirmationService],
  templateUrl: './app.html',
})
export class App implements OnDestroy {
  private readonly api = inject(ApiService);
  private readonly confirm = inject(ConfirmationService);

  readonly state = signal<AppState | null>(null);
  readonly about = signal<AboutInfo | null>(null);
  readonly status = signal<Status | null>(null);
  readonly busy = signal(false);
  readonly aboutOpen = signal(false);
  readonly searchTerm = signal('');
  readonly selectedHistory = signal<HistoryEntry | null>(null);

  /** Object URL for whatever is currently shown in the preview frame. */
  readonly previewUrl = signal<string | null>(null);
  private lastPreviewUrl: string | null = null;

  readonly detached = this.api.detached;

  /** True while a browsed or clipboard image is previewed but not applied. */
  readonly hasPending = computed(() => !!this.state()?.pending);

  readonly canApply = computed(() => this.hasPending() && !this.busy());

  /** The recommended-size note, or an empty string when it matches. */
  readonly sizeNote = computed(() => {
    const st = this.state();
    if (!st) {
      return '';
    }
    const [w, h] = st.recommended;
    const info = st.pending ? st.pending.info : st.live[0]?.info;
    if (!info) {
      return '';
    }
    if (info.width === w && info.height === h) {
      return '';
    }
    return `Recommended size is ${w}×${h}; this image is ${info.width}×${info.height}.`;
  });

  /** The previewed or live image's real pixel size, for the detail line. */
  readonly dimensions = computed(() => {
    const st = this.state();
    const info = st?.pending ? st.pending.info : st?.live[0]?.info;
    if (!info) {
      return '';
    }
    return `${info.width} × ${info.height} pixels · ${info.format.toUpperCase()}`;
  });

  readonly headline = computed(() => {
    const st = this.state();
    if (!st) {
      return 'Loading…';
    }
    if (st.pending) {
      return `Previewing: ${st.pending.name}`;
    }
    if (st.live.length > 0) {
      return st.live[0].name;
    }
    return 'No custom splash logo is set';
  });

  readonly subline = computed(() => {
    const st = this.state();
    if (!st) {
      return '';
    }
    if (st.pending) {
      const from = st.pending.source === 'clipboard' ? 'clipboard' : 'file';
      return `Imported from ${from}. Not yet applied — choose Set As Splash Logo to apply it.`;
    }
    if (st.live.length > 0) {
      const original = st.current?.originalName;
      return original ? `Original file: ${original}` : 'Original filename is not recorded.';
    }
    return 'The default ProfileUnity logo is in use.';
  });

  constructor() {
    void this.refresh();
    void this.loadAbout();
  }

  ngOnDestroy(): void {
    this.revokePreview();
  }

  private revokePreview(): void {
    if (this.lastPreviewUrl) {
      URL.revokeObjectURL(this.lastPreviewUrl);
      this.lastPreviewUrl = null;
    }
  }

  private setStatus(text: string, kind: Status['kind'] = 'good'): void {
    this.status.set({ text, kind });
  }

  private fail(error: unknown): void {
    const message =
      error instanceof ApiError || error instanceof Error ? error.message : String(error);
    this.setStatus(message, 'poor');
  }

  /** Runs an action with the busy flag held, refreshing state afterwards. */
  private async run(action: () => Promise<void>): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    try {
      await action();
    } catch (error) {
      this.fail(error);
    } finally {
      this.busy.set(false);
    }
  }

  async refresh(): Promise<void> {
    if (this.detached) {
      return;
    }
    try {
      const next = await this.api.state();
      this.state.set(next);
      await this.refreshPreview(next);
    } catch (error) {
      this.fail(error);
    }
  }

  private async refreshPreview(st: AppState): Promise<void> {
    const kind: 'pending' | 'live' | null = st.pending ? 'pending' : st.live.length ? 'live' : null;
    this.revokePreview();
    if (!kind) {
      this.previewUrl.set(null);
      return;
    }
    try {
      const url = await this.api.imageObjectUrl(kind);
      this.lastPreviewUrl = url;
      this.previewUrl.set(url);
    } catch {
      // A logo that cannot be decoded is reported through state.live[].infoError;
      // an empty frame here is the right fallback.
      this.previewUrl.set(null);
    }
  }

  private async loadAbout(): Promise<void> {
    if (this.detached) {
      return;
    }
    try {
      this.about.set(await this.api.about());
    } catch {
      // The About dialog degrades to whatever is known; not worth a status line.
    }
  }

  // --- actions --------------------------------------------------------------

  browse(): Promise<void> {
    return this.run(async () => {
      const result = await this.api.browse();
      if (result?.cancelled) {
        return;
      }
      await this.refresh();
      this.setStatus('Previewing the selected file. Not yet applied.', 'fair');
    });
  }

  importClipboard(): Promise<void> {
    return this.run(async () => {
      await this.api.importClipboard();
      await this.refresh();
      this.setStatus('Previewing the image imported from the clipboard. Not yet applied.', 'fair');
    });
  }

  discard(): Promise<void> {
    return this.run(async () => {
      await this.api.discardPending();
      await this.refresh();
      this.setStatus('Discarded the previewed image.');
    });
  }

  apply(): Promise<void> {
    return this.run(async () => {
      const result = await this.api.apply();
      await this.refresh();
      this.setStatus(`Splash logo updated: ${result.applied}`);
    });
  }

  search(): Promise<void> {
    return this.run(async () => {
      const term = this.searchTerm().trim();
      if (!term) {
        this.setStatus('Enter a search term first.', 'poor');
        return;
      }
      await this.api.search(term);
      this.setStatus(
        `Opened an image search for "${term}" in your browser. Right-click an image, choose Copy Image, then choose Import From Clipboard.`,
      );
    });
  }

  previewSplash(): Promise<void> {
    return this.run(async () => {
      await this.api.previewSplash();
      this.setStatus('Launched the splash screen preview.');
    });
  }

  restoreSelected(): void {
    const entry = this.selectedHistory();
    if (!entry) {
      this.setStatus('Select a history entry first.', 'poor');
      return;
    }
    this.confirm.confirm({
      header: 'Restore Logo',
      message: `Restore "${entry.originalName}" as the live splash logo? The current logo will be archived to history.`,
      acceptLabel: 'Restore',
      rejectLabel: 'Cancel',
      accept: () => {
        void this.run(async () => {
          const result = await this.api.restore(entry.id);
          this.selectedHistory.set(null);
          await this.refresh();
          this.setStatus(`Restored: ${result.restored}`);
        });
      },
    });
  }

  deleteSelected(): void {
    const entry = this.selectedHistory();
    if (!entry) {
      this.setStatus('Select a history entry first.', 'poor');
      return;
    }
    this.confirm.confirm({
      header: 'Delete From History',
      message: `Permanently delete "${entry.originalName}" from history? This cannot be undone.`,
      acceptLabel: 'Delete',
      rejectLabel: 'Cancel',
      accept: () => {
        void this.run(async () => {
          await this.api.deleteHistory(entry.id);
          this.selectedHistory.set(null);
          await this.refresh();
          this.setStatus('Removed from history.');
        });
      },
    });
  }

  openDocument(which: 'license' | 'sbom' | 'notices'): Promise<void> {
    return this.run(async () => {
      await this.api.openDocument(which);
    });
  }

  openAbout(): void {
    this.aboutOpen.set(true);
    void this.loadAbout();
  }
}
