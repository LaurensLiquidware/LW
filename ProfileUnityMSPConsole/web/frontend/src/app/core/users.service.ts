import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { AdminUser, UserWriteRequest } from './models/user';

@Injectable({ providedIn: 'root' })
export class UsersService {
  private readonly http = inject(HttpClient);

  list(): Promise<AdminUser[]> {
    return firstValueFrom(this.http.get<AdminUser[]>('/api/users'));
  }

  create(req: UserWriteRequest): Promise<AdminUser> {
    return firstValueFrom(this.http.post<AdminUser>('/api/users', req));
  }

  delete(id: string): Promise<void> {
    return firstValueFrom(this.http.delete<void>(`/api/users/${id}`));
  }
}
