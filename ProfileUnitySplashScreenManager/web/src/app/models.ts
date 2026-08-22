/** Shapes returned by the Go API. Kept in step with internal/api/server.go. */

export interface ImageInfo {
  width: number;
  height: number;
  format: string;
}

export interface LiveLogo {
  name: string;
  path: string;
  ext: string;
  modified: string;
  size: number;
  info: ImageInfo | null;
  infoError?: string;
}

export interface CurrentMeta {
  originalName: string;
  storedFileName: string;
  dateSet: string;
}

export interface Pending {
  path: string;
  name: string;
  info: ImageInfo;
  /** 'browse' or 'clipboard'. */
  source: string;
}

export interface HistoryEntry {
  id: string;
  originalName: string;
  extension: string;
  dateArchived: string;
  fileMissing: boolean;
}

export interface AppState {
  targetDir: string;
  targetDirExists: boolean;
  live: LiveLogo[];
  current: CurrentMeta | null;
  pending: Pending | null;
  history: HistoryEntry[];
  previewExe: string;
  previewExists: boolean;
  recommended: [number, number];
  warnings: string[];
}

export interface AboutInfo {
  product: string;
  version: string;
  company: string;
  targetDir: string;
  dataDir: string;
  exeDir: string;
  elevated: boolean;
  webViewVersion: string;
  licensePresent: boolean;
  sbomPresent: boolean;
  noticesPresent: boolean;
  licensePath: string;
  sbomPath: string;
  searchEnabled: boolean;
  searchHost: string;
}
