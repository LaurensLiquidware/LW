import { Component, OnInit, computed, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { InputTextModule } from 'primeng/inputtext';
import { PasswordModule } from 'primeng/password';
import { MessageModule } from 'primeng/message';
import { ConfirmationService } from 'primeng/api';
import { ConfirmDialogModule } from 'primeng/confirmdialog';

import { AdminUser } from '../../core/models/user';
import { UsersService } from '../../core/users.service';
import { SessionService } from '../../core/session.service';

/**
 * Console login-account management (project brief §9): create and
 * remove operator accounts. There is no edit -- an account's only
 * mutable state today is its password, changed only by that operator
 * themselves from the header's Change Password form -- so unlike
 * Tenants this screen has no separate create/edit form component, just
 * a create dialog inline here.
 *
 * Every created account is a plain operator (no role picker): nothing
 * in this app enforces any difference between operator/viewer today.
 */
@Component({
    selector: 'app-users',
    imports: [
        ReactiveFormsModule,
        TranslocoModule,
        TableModule,
        ButtonModule,
        DialogModule,
        InputTextModule,
        PasswordModule,
        MessageModule,
        ConfirmDialogModule,
    ],
    providers: [ConfirmationService],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './users.component.html',
})
export class UsersComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly users = inject(UsersService);
  private readonly session = inject(SessionService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly transloco = inject(TranslocoService);

  readonly rows = signal<AdminUser[]>([]);
  readonly loading = signal(true);
  readonly dialogVisible = signal(false);
  readonly saving = signal(false);
  readonly saveError = signal<string | null>(null);

  /** The signed-in operator's own account ID -- used to disable Delete
   * on their own row, so an operator can't lock themselves out (the
   * actual hard block is server-side; this is only the UX nicety). */
  readonly currentUserId = computed(() => this.session.user()?.id ?? null);

  readonly form = this.fb.nonNullable.group({
    username: ['', Validators.required],
    password: ['', [Validators.required, Validators.minLength(12)]],
  });

  async ngOnInit(): Promise<void> {
    await this.reload();
  }

  async reload(): Promise<void> {
    this.loading.set(true);
    try {
      this.rows.set(await this.users.list());
    } finally {
      this.loading.set(false);
    }
  }

  openCreate(): void {
    this.form.reset({ username: '', password: '' });
    this.saveError.set(null);
    this.dialogVisible.set(true);
  }

  async save(): Promise<void> {
    if (this.form.invalid || this.saving()) {
      return;
    }
    this.saving.set(true);
    this.saveError.set(null);
    const v = this.form.getRawValue();
    try {
      await this.users.create({ username: v.username, password: v.password });
      this.dialogVisible.set(false);
      await this.reload();
    } catch (err) {
      this.saveError.set(err instanceof HttpErrorResponse && err.status === 409 ? 'users.usernameTaken' : 'users.saveError');
    } finally {
      this.saving.set(false);
    }
  }

  confirmDelete(user: AdminUser): void {
    this.confirmation.confirm({
      header: this.transloco.translate('users.deleteConfirmTitle'),
      message: this.transloco.translate('users.deleteConfirmMessage', { username: user.username }),
      acceptLabel: this.transloco.translate('users.delete'),
      rejectLabel: this.transloco.translate('users.cancel'),
      accept: async () => {
        await this.users.delete(user.id);
        await this.reload();
      },
    });
  }
}
