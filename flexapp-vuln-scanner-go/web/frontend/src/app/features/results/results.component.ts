import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
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
 * already-completed scan opened from disk). Reachable with neither
 * (e.g. the sidebar's Results link) too -- that falls back to the most
 * recently created finished scan from GET /api/scans, the same list the
 * Dashboard shows, so Results is never just a dead end. */
@Component({
  selector: 'app-results',
  imports: [TranslocoModule, TableModule, TagModule, ButtonModule, CardModule, DecimalPipe, RouterLink],
  changeDetection: ChangeDetectionStrategy.Default,
  templateUrl: './results.component.html',
})
export class ResultsComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
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
  /** True only when no scan has ever finished -- distinct from
   * loadError, which covers a specific jobId/inventoryPath that failed
   * to load; this gets its own "go start one" message instead. */
  readonly noScansYet = signal(false);

  readonly allRows = computed<FindingRow[]>(() => {
    const result = this.result();
    if (!result) {
      return [];
    }
    return [...(result.confirmedRows ?? []), ...(result.heuristicRows ?? [])];
  });

  async ngOnInit(): Promise<void> {
    let jobId = this.route.snapshot.queryParamMap.get('jobId');
    let inventoryPath = this.route.snapshot.queryParamMap.get('inventoryPath');

    if (!jobId && !inventoryPath) {
      const latest = await this.findLatestFinishedScan();
      if (!latest) {
        this.noScansYet.set(true);
        this.loading.set(false);
        return;
      }
      jobId = latest.jobId ?? null;
      inventoryPath = latest.inventoryPath ?? null;
      // Reflect the resolved scan in the URL so refresh/share/back all
      // keep working, without re-running ngOnInit for this navigation.
      this.router.navigate([], {
        relativeTo: this.route,
        queryParams: jobId ? { jobId } : { inventoryPath },
        replaceUrl: true,
      });
    }

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

  /** The most recently created scan (GET /api/scans is already sorted
   * newest first) that has something to show -- done or error, live or
   * historical. Returns null if there's nothing to redirect to (no
   * scans at all, or nothing has finished yet). */
  private async findLatestFinishedScan(): Promise<{ jobId?: string; inventoryPath?: string } | null> {
    try {
      const scans = await this.scanService.listScans();
      const candidate = scans.find((s) => s.status === 'done' || s.status === 'error');
      if (!candidate) {
        return null;
      }
      if (!candidate.live) {
        return candidate.inventoryPath ? { inventoryPath: candidate.inventoryPath } : null;
      }
      return { jobId: candidate.id };
    } catch {
      return null;
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
