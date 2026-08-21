// Mirrors the JSON shapes internal/httpapi/scans.go and internal/pipeline
// produce -- see ../../../../internal/pipeline/pipeline.go (Result, Diff)
// and internal/report/report.go (FindingRow).

export interface Coverage {
  totalFilesScanned: number;
  excludedCount: number;
  excludedByReason: Record<string, number>;
  candidateComponents: number;
  resolvedComponents: number;
  resolvedByMethod: Record<string, number>;
  unresolvedComponents: number;
  unresolvedFiles: UnresolvedFile[];
  coveragePercent: number | null;
}

export interface UnresolvedFile {
  relativePath: string;
  componentType: string;
  readError: string | null;
}

export interface FindingRow {
  severityLevel: string;
  id: string;
  url?: string;
  summary: string;
  product: string;
  version: string;
  relativePaths: string[];
  confidence: string;
  source: string;
}

export interface ResultFiles {
  sbom: string;
  coverageReport: string;
  findings: string;
  pdf: string;
  findingsCsv?: string;
}

export interface ScanResult {
  packageName: string;
  coverage: Coverage;
  confirmedRows: FindingRow[] | null;
  heuristicRows: FindingRow[] | null;
  severityCounts: Record<string, number>;
  hasVulnMatches: boolean;
  inventoryPath: string;
  outputDir: string;
  files: ResultFiles;
}

export interface ScanDiff {
  old: ScanResult;
  new: ScanResult;
  newFindings: FindingRow[];
  resolvedFindings: FindingRow[];
  unchangedCount: number;
}

export type ScanStatus = 'queued' | 'stage1' | 'stage2' | 'done' | 'error';

export interface ScanSnapshot {
  id: string;
  packagePath: string;
  outputDir: string;
  status: ScanStatus;
  log: string[];
  error?: string;
  createdAt: string;
  progressPhase?: string;
  progressDone: number;
  progressTotal: number;
  result?: ScanResult;
}
