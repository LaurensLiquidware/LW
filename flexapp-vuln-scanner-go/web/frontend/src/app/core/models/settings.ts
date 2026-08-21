/**
 * Everything the Settings screen can change at runtime, applied
 * immediately (no restart) and persisted so it survives one. Anything
 * needed just to boot the process (listen address, DB driver/DSN, the
 * credential encryption key, the initial admin account) stays env-var
 * only and never appears here.
 */
export interface Settings {
  smtpHost: string;
  smtpPort: number;
  smtpUsername: string;
  /** Never the actual password -- only whether one is currently set. */
  smtpPasswordSet: boolean;
  smtpFrom: string;
  smtpSecurity: 'starttls' | 'tls' | 'none';

  reportRecipients: string[];
  reportEmailDay: number;

  collectionIntervalSeconds: number;
  collectionTimezone: string;
  collectionConcurrency: number;
  collectionTenantTimeoutSeconds: number;

  sessionIdleTimeoutSeconds: number;
  sessionAbsoluteTimeoutSeconds: number;

  tlsCertSubject?: string;
  tlsCertExpiresUtc?: string;
  tlsCertSelfSigned: boolean;
  tlsCertConfigured: boolean;
}

/**
 * smtpPassword mirrors the backend's three-way semantics (same as a
 * tenant's password): omitted (undefined) leaves the stored password
 * untouched; a non-empty string sets it. There's no "clear" case here --
 * unlike a tenant, SMTP either has a password or the relay doesn't need
 * one, and typing an empty string in the form means "no change", not
 * "clear it" (use the dedicated field state to represent that instead).
 */
export interface SettingsWriteRequest {
  smtpHost: string;
  smtpPort: number;
  smtpUsername: string;
  smtpPassword?: string;
  smtpFrom: string;
  smtpSecurity: 'starttls' | 'tls' | 'none';

  reportRecipients: string[];
  reportEmailDay: number;

  collectionIntervalSeconds: number;
  collectionTimezone: string;
  collectionConcurrency: number;
  collectionTenantTimeoutSeconds: number;

  sessionIdleTimeoutSeconds: number;
  sessionAbsoluteTimeoutSeconds: number;
}

export interface TlsCertUploadRequest {
  certPem: string;
  keyPem: string;
}

export interface TestEmailRequest {
  smtpHost: string;
  smtpPort: number;
  smtpUsername: string;
  smtpPassword?: string;
  smtpFrom: string;
  smtpSecurity: 'starttls' | 'tls' | 'none';
  to: string;
}
