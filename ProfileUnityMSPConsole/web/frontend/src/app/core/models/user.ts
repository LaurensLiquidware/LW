/**
 * A console login account (project brief §9) -- create/list/remove only,
 * from the Users screen. Every account created there is a plain
 * operator; there is no role picker since nothing in this app currently
 * enforces a distinction between operator and viewer.
 */
export interface AdminUser {
  id: string;
  username: string;
  role: string;
  createdAtUtc: string;
  updatedAtUtc: string;
}

export interface UserWriteRequest {
  username: string;
  password: string;
}
