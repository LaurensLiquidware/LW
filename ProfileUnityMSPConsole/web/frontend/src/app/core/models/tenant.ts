export interface Tenant {
  id: string;
  displayName: string;
  hostname: string;
  port: number;
  username: string;
  hasPassword: boolean;
  tlsSkipVerify: boolean;
  enabled: boolean;
  tags: string[];
  notes: string;
  createdAtUtc: string;
  updatedAtUtc: string;
}

/**
 * password mirrors the backend's three-way semantics: omitted (undefined)
 * leaves a stored password untouched on update / means "no password" on
 * create; null clears it; a non-empty string sets it.
 */
export interface TenantWriteRequest {
  displayName: string;
  hostname: string;
  port: number;
  username: string;
  password?: string | null;
  tlsSkipVerify: boolean;
  enabled: boolean;
  tags: string[];
  notes: string;
}

export interface TestConnectionRequest {
  hostname: string;
  port: number;
  tlsSkipVerify: boolean;
  username: string;
  password?: string | null;
}

export type ConnectivityOutcome =
  | 'unauthenticated_success'
  | 'authenticated_success'
  | 'tls_failure'
  | 'timeout'
  | 'auth_rejected'
  | 'auth_required'
  | 'malformed_response'
  | 'unreachable'
  | 'error';

export interface TestConnectionResponse {
  outcome: ConnectivityOutcome;
  message: string;
}
