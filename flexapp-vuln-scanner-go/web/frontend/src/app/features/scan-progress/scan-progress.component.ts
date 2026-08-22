import { Component, OnDestroy, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { ProgressBarModule } from 'primeng/progressbar';
import { TagModule } from 'primeng/tag';
import { ButtonModule } from 'primeng/button';

import { ScanService } from '../../core/scan.service';
import { ScanSnapshot } from '../../core/models/scan';

const TERMINAL_STATUSES = new Set(['done', 'error', 'canceled']);

/** Streams one scan job's live log + progress over Server-Sent Events
 * (GET /api/scans/{id}/events) until it reaches "done", "error", or
 * "canceled", then hands off to the Results screen automatically on
 * "done". Falls back to polling GET /api/scans/{id} if the stream
 * itself fails to open (e.g. an intermediary that doesn't support
 * SSE) -- the browser's own EventSource already retries a dropped
 * connection on its own, so this fallback only covers the "never
 * connected in the first place" case. */
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
  private eventSource: EventSource | undefined;
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
    this.connect();
  }

  ngOnDestroy(): void {
    this.eventSource?.close();
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

  private connect(): void {
    const es = new EventSource(this.scanService.eventsUrl(this.jobId));
    this.eventSource = es;
    let receivedAny = false;

    es.onmessage = (event) => {
      receivedAny = true;
      const job = JSON.parse(event.data) as ScanSnapshot;
      this.handleUpdate(job);
    };

    es.onerror = () => {
      if (!receivedAny) {
        // Never connected at all (e.g. something between the browser
        // and the server doesn't support SSE) -- fall back to polling
        // rather than leaving the screen stuck with no updates.
        es.close();
        this.eventSource = undefined;
        this.startPolling();
      }
      // A drop after at least one message: EventSource retries the
      // connection on its own, so nothing to do here.
    };
  }

  private handleUpdate(job: ScanSnapshot): void {
    this.job.set(job);
    if (TERMINAL_STATUSES.has(job.status)) {
      this.eventSource?.close();
      if (this.pollHandle) {
        clearInterval(this.pollHandle);
      }
      if (job.status === 'done') {
        this.router.navigate(['/results'], { queryParams: { jobId: job.id } });
      }
    }
  }

  private startPolling(): void {
    const poll = async () => {
      try {
        this.handleUpdate(await this.scanService.getScan(this.jobId));
      } catch {
        // Transient network hiccup while polling -- try again next tick.
      }
    };
    poll();
    this.pollHandle = setInterval(poll, 1000);
  }
}
