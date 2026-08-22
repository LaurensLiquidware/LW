import { Component, Input, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { TranslocoModule } from '@jsverse/transloco';
import { CardModule } from 'primeng/card';

import { VersionService } from '../../core/version.service';

/**
 * About screen: version plus the license/SBOM pointers and disclaimer
 * text the project brief §11.7 requires surfacing where the user can
 * actually see them, not just in the source tree. Reachable without
 * signing in (as its own route), since the license should be presentable
 * before first use -- and reused as the content of a dialog once signed
 * in (see shell.component.html), so it never needs its own back button.
 */
@Component({
    selector: 'app-about',
    imports: [TranslocoModule, CardModule],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './about.component.html'
})
export class AboutComponent {
  /** False when hosted inside a dialog that already shows its own
   * "About" title -- avoids showing the heading twice. */
  @Input() showHeading = true;

  private readonly versionService = inject(VersionService);
  readonly version = signal<string | null>(null);

  constructor() {
    this.versionService
      .fetch()
      .then((version) => this.version.set(version))
      .catch(() => this.version.set(null));
  }
}
