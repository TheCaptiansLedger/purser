// formatBytes renders a byte count the way the Database tab's (#613) info
// cards and progress indicators need it — no shared byte-formatting
// helper exists elsewhere in web/src to reuse. Uses binary (1024) units,
// one decimal place above KB, matching the pre-reset DatabasePage's own
// convention (prior art for this tab's UX, not resurrected code).
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`
}
