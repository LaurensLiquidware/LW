import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { TranslocoModule } from '@jsverse/transloco';
import { CardModule } from 'primeng/card';

/**
 * About screen: version plus the license/SBOM pointers and disclaimer
 * text the project brief §11.7 requires surfacing where the user can
 * actually see them, not just in the source tree. Reachable without
 * signing in, since the license should be presentable before first use.
 */
@Component({
    selector: 'app-about',
    imports: [TranslocoModule, CardModule],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './about.component.html'
})
export class AboutComponent {
  private readonly http = inject(HttpClient);
  readonly version = signal<string | null>(null);

  constructor() {
    this.http.get<{ version: string }>('/api/version').subscribe({
      next: (res) => this.version.set(res.version),
      error: () => this.version.set(null),
    });
  }
}
