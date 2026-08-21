import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { PanelModule } from 'primeng/panel';

import { ScanService } from '../../core/scan.service';

/** New Scan form: package path + output folder (auto-filled from the
 * package's file stem, same "default output folder" behavior the
 * Flask web UI had), plus an Advanced section for the NVD API key. */
@Component({
  selector: 'app-new-scan',
  imports: [ReactiveFormsModule, TranslocoModule, ButtonModule, InputTextModule, MessageModule, PanelModule],
  changeDetection: ChangeDetectionStrategy.Default,
  templateUrl: './new-scan.component.html',
})
export class NewScanComponent {
  private readonly fb = inject(FormBuilder);
  private readonly scanService = inject(ScanService);
  private readonly router = inject(Router);

  readonly submitting = signal(false);
  readonly error = signal<string | null>(null);
  /** Whether error holds a raw backend message (shown verbatim) rather
   * than a translation key to pipe through transloco. */
  readonly errorIsRaw = signal(false);

  readonly form = this.fb.nonNullable.group({
    packagePath: ['', Validators.required],
    outputDir: ['./scan-out', Validators.required],
    nvdApiKey: [''],
  });

  onPackagePathChange(): void {
    const packagePath = this.form.controls.packagePath.value;
    const outputDirTouched = this.form.controls.outputDir.dirty;
    if (!packagePath || outputDirTouched) {
      return;
    }
    const stem = packagePath.replace(/\\/g, '/').split('/').pop()?.replace(/\.[^.]+$/, '') ?? packagePath;
    this.form.controls.outputDir.setValue(`./scan-out/${stem}`);
  }

  async submit(): Promise<void> {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.submitting.set(true);
    this.error.set(null);
    this.errorIsRaw.set(false);
    const { packagePath, outputDir, nvdApiKey } = this.form.getRawValue();
    try {
      const job = await this.scanService.startScan(packagePath, outputDir, nvdApiKey || undefined);
      this.router.navigate(['/scan-progress'], { queryParams: { jobId: job.id } });
    } catch (err) {
      if (err instanceof HttpErrorResponse && err.error?.error) {
        this.error.set(err.error.error);
        this.errorIsRaw.set(true);
      } else {
        this.error.set('newScan.genericError');
      }
      this.submitting.set(false);
    }
  }
}
