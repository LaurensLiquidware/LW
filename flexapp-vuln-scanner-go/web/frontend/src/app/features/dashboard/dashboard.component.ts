import { Component, OnDestroy, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DatePipe, DecimalPipe } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';

import { ScanService } from '../../core/scan.service';
import { ScanSnapshot, ScanStatus } from '../../core/models/scan';

const STATUS_SEVERITY: Record<ScanStatus, 'success' | 'danger' | 'info' | 'warn'> = {
  queued: 'info',
  stage1: 'info',
  stage2: 'info',
  done: 'success',
  error: 'danger',
};

/** Dashboard: every scan job started this process's lifetime, plus
 * scan history persisted across restarts (internal/scanstore), newest
 * first. Polls so status/coverage updates without a manual refresh.
 * A historical row (scan.live === false) has no live job to poll --
 * opening it re-reads the real files via its inventoryPath instead. */
@Component({
  selector: 'app-dashboard',
  imports: [RouterLink, TranslocoModule, TableModule, ButtonModule, TagModule, DatePipe, DecimalPipe],
  changeDetection: ChangeDetectionStrategy.Default,
  templateUrl: './dashboard.component.html',
})
export class DashboardComponent implements OnInit, OnDestroy {
  private readonly scanService = inject(ScanService);
  private readonly router = inject(Router);
  private pollHandle: ReturnType<typeof setInterval> | undefined;

  readonly scans = signal<ScanSnapshot[]>([]);
  readonly loading = signal(true);

  async ngOnInit(): Promise<void> {
    await this.load();
    this.loading.set(false);
    this.pollHandle = setInterval(() => this.load(), 2000);
  }

  ngOnDestroy(): void {
    if (this.pollHandle) {
      clearInterval(this.pollHandle);
    }
  }

  private async load(): Promise<void> {
    try {
      this.scans.set(await this.scanService.listScans());
    } catch {
      // Transient network hiccup while polling -- keep showing the last
      // known list rather than clearing it.
    }
  }

  severityFor(status: ScanStatus): 'success' | 'danger' | 'info' | 'warn' {
    return STATUS_SEVERITY[status] ?? 'info';
  }

  open(scan: ScanSnapshot): void {
    if (!scan.live) {
      // No live job to poll in this process -- only a completed scan's
      // inventory (and therefore its report files) can still be shown.
      if (scan.inventoryPath) {
        this.router.navigate(['/results'], { queryParams: { inventoryPath: scan.inventoryPath } });
      }
      return;
    }
    if (scan.status === 'done' || scan.status === 'error') {
      this.router.navigate(['/results'], { queryParams: { jobId: scan.id } });
    } else {
      this.router.navigate(['/scan-progress'], { queryParams: { jobId: scan.id } });
    }
  }
}
