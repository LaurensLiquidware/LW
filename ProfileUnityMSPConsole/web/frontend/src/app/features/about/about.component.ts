import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { TranslocoModule } from '@jsverse/transloco';
import { CardModule } from 'primeng/card';

import { VersionService } from '../../core/version.service';

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
  private readonly versionService = inject(VersionService);
  readonly version = signal<string | null>(null);

  constructor() {
    this.versionService
      .fetch()
      .then((version) => this.version.set(version))
      .catch(() => this.version.set(null));
  }
}
