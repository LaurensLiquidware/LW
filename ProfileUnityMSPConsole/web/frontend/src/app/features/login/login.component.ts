import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { MessageModule } from 'primeng/message';

import { SessionService } from '../../core/session.service';
import { LanguageSwitcherComponent } from '../../shared/language-switcher.component';

@Component({
    selector: 'app-login',
    imports: [
        ReactiveFormsModule,
        TranslocoModule,
        ButtonModule,
        MessageModule,
        LanguageSwitcherComponent,
    ],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './login.component.html',
    styleUrl: './login.component.css'
})
export class LoginComponent {
  private readonly fb = inject(FormBuilder);
  private readonly session = inject(SessionService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  readonly form = this.fb.nonNullable.group({
    username: ['', Validators.required],
    password: ['', Validators.required],
  });

  readonly submitting = signal(false);
  readonly errorKey = signal<string | null>(null);
  readonly expired = signal(this.route.snapshot.queryParamMap.get('expired') === '1');
  readonly passwordVisible = signal(false);

  async submit(): Promise<void> {
    if (this.form.invalid || this.submitting()) {
      return;
    }
    this.submitting.set(true);
    this.errorKey.set(null);
    const { username, password } = this.form.getRawValue();
    try {
      await this.session.login(username, password);
      await this.router.navigateByUrl('/');
    } catch (err) {
      const status = err instanceof HttpErrorResponse ? err.status : 0;
      this.errorKey.set(status === 401 ? 'login.invalidCredentials' : 'login.genericError');
    } finally {
      this.submitting.set(false);
    }
  }
}
