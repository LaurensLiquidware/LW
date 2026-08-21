import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { ScanDiff, ScanResult, ScanSnapshot } from './models/scan';

/** Thin wrapper over the /api/scans REST API (internal/httpapi/scans.go) --
 * every method here maps to exactly one endpoint, no client-side business
 * logic duplicated from the Go pipeline. */
@Injectable({ providedIn: 'root' })
export class ScanService {
  private readonly http = inject(HttpClient);

  startScan(packagePath: string, outputDir: string, nvdApiKey?: string): Promise<ScanSnapshot> {
    return firstValueFrom(
      this.http.post<ScanSnapshot>('/api/scans', { packagePath, outputDir, nvdApiKey: nvdApiKey || undefined }),
    );
  }

  refreshScan(inventoryPath: string, outputDir: string, nvdApiKey?: string): Promise<ScanSnapshot> {
    return firstValueFrom(
      this.http.post<ScanSnapshot>('/api/scans/refresh', {
        inventoryPath,
        outputDir,
        nvdApiKey: nvdApiKey || undefined,
      }),
    );
  }

  listScans(): Promise<ScanSnapshot[]> {
    return firstValueFrom(this.http.get<ScanSnapshot[]>('/api/scans'));
  }

  getScan(id: string): Promise<ScanSnapshot> {
    return firstValueFrom(this.http.get<ScanSnapshot>(`/api/scans/${id}`));
  }

  cancelScan(id: string): Promise<ScanSnapshot> {
    return firstValueFrom(this.http.post<ScanSnapshot>(`/api/scans/${id}/cancel`, {}));
  }

  openScan(inventoryPath: string): Promise<ScanResult> {
    return firstValueFrom(this.http.post<ScanResult>('/api/scans/open', { inventoryPath }));
  }

  compareScans(oldDir: string, newDir: string): Promise<ScanDiff> {
    return firstValueFrom(this.http.post<ScanDiff>('/api/scans/compare', { oldDir, newDir }));
  }

  /** URL for downloading a completed job's report artifact -- used
   * directly as an <a href> so the browser/OS handles the download,
   * no client-side blob handling needed. */
  downloadUrl(jobId: string, kind: 'sbom' | 'coverage' | 'findings' | 'pdf' | 'csv'): string {
    return `/api/scans/${jobId}/files/${kind}`;
  }
}
