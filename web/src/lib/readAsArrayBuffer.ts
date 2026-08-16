// readAsArrayBuffer wraps FileReader in a Promise — used instead of the
// newer Blob.prototype.arrayBuffer() because jsdom's Blob/File polyfill
// (the test environment ADR 0004 runs against) doesn't implement it, and
// piping a jsdom Blob through fetch's Response silently truncates it too
// (Response's own Blob detection doesn't recognize jsdom's Blob class).
// FileReader is the one path that behaves the same under jsdom and real
// browsers, so tests exercise the same code a browser runs. Shared by
// every caller that needs a File/Blob's bytes client-side
// (useDatabaseRestore's upload chunking, useAttachImage's file-upload
// entry point) rather than each reimplementing it.
export function readAsArrayBuffer(blob: Blob): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.onerror = () => reject(reader.error as DOMException)
    reader.readAsArrayBuffer(blob)
  })
}
