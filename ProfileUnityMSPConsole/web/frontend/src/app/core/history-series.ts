/** One (x, y) sample where a missing/non-success day is represented as
 * `value: null` — the caller decides what counts as a gap. */
export interface SeriesInput {
  date: string;
  value: number | null;
}

export interface Series {
  labels: string[];
  values: (number | null)[];
}

/**
 * Expands sparse points into one entry per calendar day between the
 * earliest and latest date, filling any day with no matching point with
 * null. Chart.js (with spanGaps: false, the default) renders a null
 * point as a visible break in the line — never interpolated across, and
 * never drawn as zero (project brief §7.4/§2).
 */
export function buildContinuousSeries(points: SeriesInput[]): Series {
  if (points.length === 0) {
    return { labels: [], values: [] };
  }

  const byDate = new Map(points.map((p) => [p.date, p.value]));
  const dates = points.map((p) => p.date).sort();
  const start = new Date(dates[0] + 'T00:00:00Z');
  const end = new Date(dates[dates.length - 1] + 'T00:00:00Z');

  const labels: string[] = [];
  const values: (number | null)[] = [];
  for (let cursor = start; cursor <= end; cursor = new Date(cursor.getTime() + 86400000)) {
    const iso = cursor.toISOString().slice(0, 10);
    labels.push(iso);
    values.push(byDate.has(iso) ? byDate.get(iso)! : null);
  }
  return { labels, values };
}
