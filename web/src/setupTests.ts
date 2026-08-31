import '@testing-library/jest-dom/vitest'

// jsdom doesn't implement HTMLMediaElement.play/pause/load — every page now
// mounts one real <audio> element via PlayerProvider (Layout.tsx, #749), so
// this is a global stub rather than something each test file repeats.
// Individual tests that assert on play()/pause() calls (PlayerProvider.test.tsx)
// still install their own vi.fn() spy in a beforeEach to inspect call args;
// this default just keeps every other test's console quiet.
if (typeof window !== 'undefined') {
  window.HTMLMediaElement.prototype.play = () => Promise.resolve()
  window.HTMLMediaElement.prototype.pause = () => {}
  window.HTMLMediaElement.prototype.load = () => {}
}
