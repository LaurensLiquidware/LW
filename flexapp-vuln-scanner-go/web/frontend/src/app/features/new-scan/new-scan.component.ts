import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { CheckboxModule } from 'primeng/checkbox';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';
import { PanelModule } from 'primeng/panel';

import { ScanService } from '../../core/scan.service';
import { PickerService } from '../../core/picker.service';

/** Win32 FileDialog filter string for the package-path Browse button --
 * FlexApp packages ship as classic VHDX or Package Manager ZIP files. */
const PACKAGE_FILTER = 'FlexApp Packages (*.vhdx;*.zip)|*.vhdx;*.zip|All Files (*.*)|*.*';

/** New Scan form: package path + output folder (auto-filled from the
 * package's file stem, same "default output folder" behavior the
 * Flask web UI had), plus an Advanced section for the NVD API key. */
@Component({
  selector: 'app-new-scan',
  imports: [ReactiveFormsModule, TranslocoModule, ButtonModule, CheckboxModule, InputTextModule, MessageModule, PanelModule],
  changeDetection: ChangeDetectionStrategy.Default,
  templateUrl: './new-scan.component.html',
})
export class NewScanComponent {
  private readonly fb = inject(FormBuilder);
  private readonly scanService = inject(ScanService);
  private readonly pickerService = inject(PickerService);
  private readonly router = inject(Router);

  readonly submitting = signal(false);
  readonly error = signal<string | null>(null);
  /** Whether error holds a raw backend message (shown verbatim) rather
   * than a translation key to pipe through transloco. */
  readonly errorIsRaw = signal(false);

  /** Whether the tray launcher's native picker is reachable -- the
   * Browse buttons only render once this is true, so this screen
   * degrades to plain text entry when run without the tray launcher
   * (e.g. this Linux dev environment, or the server started directly). */
  readonly pickerAvailable = this.pickerService.available;

  readonly form = this.fb.nonNullable.group({
    packagePath: ['', Validators.required],
    outputDir: ['./scan-out', Validators.required],
    nvdApiKey: [''],
    // Checked (scan runs) by default -- unchecking is the opt-out for
    // this one run, e.g. a machine deliberately running a different
    // antivirus product, or just to save the extra scan time.
    runDefenderScan: [true],
  });

  constructor() {
    this.pickerService.checkAvailable();
  }

  async browsePackagePath(): Promise<void> {
    const path = await this.pickerService.pickFile({ title: 'Select a FlexApp Package', filter: PACKAGE_FILTER });
    if (path) {
      this.form.controls.packagePath.setValue(path);
      this.form.controls.packagePath.markAsDirty();
      this.onPackagePathChange();
    }
  }

  async browseOutputDir(): Promise<void> {
    const path = await this.pickerService.pickFolder({ title: 'Select an Output Folder' });
    if (path) {
      this.form.controls.outputDir.setValue(path);
      this.form.controls.outputDir.markAsDirty();
    }
  }

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
    const { packagePath, outputDir, nvdApiKey, runDefenderScan } = this.form.getRawValue();
    try {
      const job = await this.scanService.startScan(packagePath, outputDir, nvdApiKey || undefined, !runDefenderScan);
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
