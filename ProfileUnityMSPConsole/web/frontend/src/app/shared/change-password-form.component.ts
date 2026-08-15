import { Component, EventEmitter, Output, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { PasswordModule } from 'primeng/password';
import { MessageModule } from 'primeng/message';

import { SessionService } from '../core/session.service';

/**
 * Lets the signed-in operator change their own password. No admin-reset
 * path exists (project brief §9's own auth is intentionally minimal) --
 * this is the only way a password ever changes after the initial
 * PUMC_BOOTSTRAP_ADMIN_PASSWORD.
 */
@Component({
    selector: 'app-change-password-form',
    imports: [ReactiveFormsModule, TranslocoModule, ButtonModule, PasswordModule, MessageModule],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './change-password-form.component.html'
})
export class ChangePasswordFormComponent {
  private readonly fb = inject(FormBuilder);
  private readonly session = inject(SessionService);

  @Output() saved = new EventEmitter<void>();
  @Output() cancelled = new EventEmitter<void>();

  readonly form = this.fb.nonNullable.group({
    currentPassword: ['', Validators.required],
    newPassword: ['', [Validators.required, Validators.minLength(12)]],
    confirmPassword: ['', Validators.required],
  });

  readonly saving = signal(false);
  readonly saveError = signal<string | null>(null);

  async save(): Promise<void> {
    if (this.form.invalid || this.saving()) {
      return;
    }
    const { currentPassword, newPassword, confirmPassword } = this.form.getRawValue();
    if (newPassword !== confirmPassword) {
      this.saveError.set('changePassword.mismatch');
      return;
    }

    this.saving.set(true);
    this.saveError.set(null);
    try {
      await this.session.changePassword(currentPassword, newPassword);
      this.form.reset({ currentPassword: '', newPassword: '', confirmPassword: '' });
      this.saved.emit();
    } catch (err) {
      this.saveError.set(err instanceof HttpErrorResponse && err.status === 403 ? 'changePassword.wrongCurrent' : 'changePassword.error');
    } finally {
      this.saving.set(false);
    }
  }
}
