import { Component, OnDestroy, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { ProgressBarModule } from 'primeng/progressbar';
import { TagModule } from 'primeng/tag';
import { ButtonModule } from 'primeng/button';

import { ScanService } from '../../core/scan.service';
import { ScanSnapshot } from '../../core/models/scan';

/** Polls one scan job (GET /api/scans/{id}) and shows its live log +
 * progress until it reaches "done", "error", or "canceled", then hands
 * off to the Results screen automatically on "done". No SSE yet -- see
 * CHANGELOG.md. */
@Component({
  selector: 'app-scan-progress',
  imports: [TranslocoModule, ProgressBarModule, TagModule, ButtonModule],
  changeDetection: ChangeDetectionStrategy.Default,
  templateUrl: './scan-progress.component.html',
})
export class ScanProgressComponent implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly scanService = inject(ScanService);
  private pollHandle: ReturnType<typeof setInterval> | undefined;
  private jobId = '';

  readonly job = signal<ScanSnapshot | null>(null);
  readonly canceling = signal(false);

  ngOnInit(): void {
    this.jobId = this.route.snapshot.queryParamMap.get('jobId') ?? '';
    if (!this.jobId) {
      this.router.navigate(['/dashboard']);
      return;
    }
    this.poll();
    this.pollHandle = setInterval(() => this.poll(), 1000);
  }

  ngOnDestroy(): void {
    if (this.pollHandle) {
      clearInterval(this.pollHandle);
    }
  }

  progressPercent(): number {
    const job = this.job();
    if (!job || !job.progressTotal) {
      return 0;
    }
    return Math.round((job.progressDone / job.progressTotal) * 100);
  }

  isRunning(): boolean {
    const status = this.job()?.status;
    return status === 'queued' || status === 'stage1' || status === 'stage2';
  }

  async cancel(): Promise<void> {
    this.canceling.set(true);
    try {
      this.job.set(await this.scanService.cancelScan(this.jobId));
    } finally {
      this.canceling.set(false);
    }
  }

  private async poll(): Promise<void> {
    try {
      const job = await this.scanService.getScan(this.jobId);
      this.job.set(job);
      if (job.status === 'done' || job.status === 'error' || job.status === 'canceled') {
        if (this.pollHandle) {
          clearInterval(this.pollHandle);
        }
        if (job.status === 'done') {
          this.router.navigate(['/results'], { queryParams: { jobId: job.id } });
        }
      }
    } catch {
      // Transient network hiccup while polling -- try again next tick.
    }
  }
}
