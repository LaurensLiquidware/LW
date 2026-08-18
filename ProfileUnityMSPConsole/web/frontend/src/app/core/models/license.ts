/**
 * A tenant's ProfileUnity License Server connection -- a distinct host
 * from the tenant's own console, authenticated with that server's own
 * Mongo-connection-string username/password (its only identity store).
 */
export interface LicenseServerConnection {
  hostname: string;
  port: number;
  username: string;
  /** Never the actual password -- only whether one is currently set. */
  hasPassword: boolean;
  tlsSkipVerify: boolean;
}

/**
 * password mirrors the tenant's own credential update semantics:
 * omitted (undefined) leaves the stored password untouched, an empty
 * string clears it, a non-empty string replaces it.
 */
export interface LicenseServerWriteRequest {
  hostname: string;
  port: number;
  username: string;
  password?: string;
  tlsSkipVerify: boolean;
}

/** Fields decoded locally from a signed license code, for review before
 * pushing -- never confirms the license is genuinely signed (only the
 * target License Server's own signature check does that). */
export interface DecodedLicense {
  organization: string;
  contactName: string;
  contactEmail: string;
  validUntil: string;
  licenseType: string;
  maxUsers: number;
  isMachine: boolean;
  isConcurrent: boolean;
}

export type LicensePushOutcome = 'success' | 'auth_failed' | 'rejected' | 'unreachable' | 'error';

export interface LicensePushResult {
  outcome: LicensePushOutcome;
  message: string;
  fields: DecodedLicense;
}

/** One row of push history -- the License Server itself keeps no record
 * of what it replaced, so this is the only "what happened, when, by
 * whom" trail. */
export interface LicensePushRecord {
  id: string;
  pushedAtUtc: string;
  operatorUsername: string;
  outcome: LicensePushOutcome;
  errorMessage: string;
  licenseCodeBase64: string;
  organization: string;
  contactName: string;
  contactEmail: string;
  validUntil: string;
  licenseType: string;
  maxUsers: number | null;
  isMachine: boolean;
  isConcurrent: boolean;
}
