import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { TableModule } from 'primeng/table';
import { TagModule } from 'primeng/tag';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';

import { ScanService } from '../../core/scan.service';
import { FindingRow, ScanResult } from '../../core/models/scan';

const SEVERITY_SEVERITY: Record<string, 'success' | 'danger' | 'info' | 'warn' | 'secondary'> = {
  CRITICAL: 'danger',
  HIGH: 'danger',
  MEDIUM: 'warn',
  MODERATE: 'warn',
  LOW: 'secondary',
};

/** Results screen: coverage summary, severity counts, and the findings
 * table -- one row per distinct (component, vulnerability), with every
 * file sharing that finding collapsed into an expandable "Affected
 * Files" disclosure rather than repeating rows (see
 * ../../../../../flexapp-vuln-scanner/webui/templates/_macros.html's
 * affected_files_cell, which this mirrors). Reachable either with
 * ?jobId= (a scan this process ran) or ?inventoryPath= (an
 * already-completed scan opened from disk). */
@Component({
  selector: 'app-results',
  imports: [TranslocoModule, TableModule, TagModule, ButtonModule, CardModule, DecimalPipe],
  changeDetection: ChangeDetectionStrategy.Default,
  templateUrl: './results.component.html',
})
export class ResultsComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly scanService = inject(ScanService);

  readonly result = signal<ScanResult | null>(null);
  readonly jobId = signal<string | null>(null);
  readonly loading = signal(true);
  readonly loadError = signal<string | null>(null);
  /** Whether loadError holds a raw backend message (already in English,
   * shown verbatim) rather than a translation key to pipe through
   * transloco -- same convention ProfileUnityMSPConsole's settings
   * screen uses for its saveError/saveErrorIsRaw pair. */
  readonly loadErrorIsRaw = signal(false);

  readonly allRows = computed<FindingRow[]>(() => {
    const result = this.result();
    if (!result) {
      return [];
    }
    return [...(result.confirmedRows ?? []), ...(result.heuristicRows ?? [])];
  });

  async ngOnInit(): Promise<void> {
    const jobId = this.route.snapshot.queryParamMap.get('jobId');
    const inventoryPath = this.route.snapshot.queryParamMap.get('inventoryPath');

    try {
      if (jobId) {
        this.jobId.set(jobId);
        const job = await this.scanService.getScan(jobId);
        if (job.result) {
          this.result.set(job.result);
        } else if (job.error) {
          this.loadError.set(job.error);
          this.loadErrorIsRaw.set(true);
        }
      } else if (inventoryPath) {
        this.result.set(await this.scanService.openScan(inventoryPath));
      } else {
        this.loadError.set('results.noScanSpecified');
      }
    } catch {
      this.loadError.set('results.loadError');
    } finally {
      this.loading.set(false);
    }
  }

  severityFor(level: string): 'success' | 'danger' | 'info' | 'warn' | 'secondary' {
    return SEVERITY_SEVERITY[(level || '').toUpperCase()] ?? 'secondary';
  }

  downloadUrl(kind: 'sbom' | 'coverage' | 'findings' | 'pdf' | 'csv'): string | null {
    const jobId = this.jobId();
    return jobId ? this.scanService.downloadUrl(jobId, kind) : null;
  }
}
