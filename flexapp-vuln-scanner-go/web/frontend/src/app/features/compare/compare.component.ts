import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { HttpErrorResponse } from '@angular/common/http';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { TagModule } from 'primeng/tag';

import { ScanService } from '../../core/scan.service';
import { ScanDiff } from '../../core/models/scan';

/** Compare two single-package scan output directories: which findings
 * are new, which were resolved, and how many are unchanged -- mirrors
 * the desktop app's Compare dialog and internal/pipeline's LoadDiff. */
@Component({
  selector: 'app-compare',
  imports: [ReactiveFormsModule, TranslocoModule, ButtonModule, InputTextModule, MessageModule, TagModule],
  changeDetection: ChangeDetectionStrategy.Default,
  templateUrl: './compare.component.html',
})
export class CompareComponent {
  private readonly fb = inject(FormBuilder);
  private readonly scanService = inject(ScanService);

  readonly loading = signal(false);
  readonly error = signal<string | null>(null);
  /** Whether error holds a raw backend message (shown verbatim) rather
   * than a translation key to pipe through transloco. */
  readonly errorIsRaw = signal(false);
  readonly diff = signal<ScanDiff | null>(null);

  readonly form = this.fb.nonNullable.group({
    oldDir: ['', Validators.required],
    newDir: ['', Validators.required],
  });

  async submit(): Promise<void> {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.errorIsRaw.set(false);
    this.diff.set(null);
    const { oldDir, newDir } = this.form.getRawValue();
    try {
      this.diff.set(await this.scanService.compareScans(oldDir, newDir));
    } catch (err) {
      if (err instanceof HttpErrorResponse && err.error?.error) {
        this.error.set(err.error.error);
        this.errorIsRaw.set(true);
      } else {
        this.error.set('compare.genericError');
      }
    } finally {
      this.loading.set(false);
    }
  }
}
