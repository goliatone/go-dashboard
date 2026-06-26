import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

async function loadJSDOM() {
  try {
    return await import('jsdom');
  } catch (error) {
    const here = dirname(fileURLToPath(import.meta.url));
    const candidates = [
      resolve(here, '../../../../../go-formgen/client/node_modules/jsdom/lib/api.js'),
      resolve(here, '../../../../../../go-formgen/client/node_modules/jsdom/lib/api.js'),
    ];
    for (const candidate of candidates) {
      try {
        return await import(pathToFileURL(candidate).href);
      } catch (candidateError) {}
    }
    throw error;
  }
}

const require = createRequire(import.meta.url);
const shell = require('./shell.js');
const { JSDOM } = await loadJSDOM();

function setGlobals(win) {
  globalThis.window = win;
  globalThis.document = win.document;
  globalThis.HTMLElement = win.HTMLElement;
  globalThis.Event = win.Event;
  globalThis.KeyboardEvent = win.KeyboardEvent;
  globalThis.MouseEvent = win.MouseEvent;
}

function memStorage(initial = {}) {
  const map = new Map(Object.entries(initial));
  return {
    map,
    getItem: (key) => (map.has(key) ? map.get(key) : null),
    setItem: (key, value) => map.set(key, String(value)),
    removeItem: (key) => map.delete(key),
  };
}

function markup() {
  return `
    <section data-dashboard-shell data-dashboard-shell-surface="settings" data-dashboard-shell-version="1" data-dashboard-shell-namespace="go-dashboard:shell">
      <button type="button" data-shell-toggle="nav" aria-expanded="true">Nav</button>
      <button type="button" data-shell-focus-toggle="main" aria-pressed="false">Focus</button>
      <button type="button" data-shell-focus-exit aria-pressed="false">Exit</button>
      <aside data-shell-region="nav" data-shell-rail="nav" data-collapsed="false"
        data-shell-resizable data-shell-edge="trailing" data-shell-min="220" data-shell-max="420" data-shell-default-size="280"></aside>
      <div data-shell-resize="nav" role="separator" tabindex="0" aria-valuemin="220" aria-valuemax="420" aria-valuenow="280"></div>
      <main data-shell-region="main" data-shell-focus-target="main"></main>
    </section>`;
}

function setup(html = markup()) {
  const dom = new JSDOM(`<!doctype html><html><body>${html}</body></html>`, { url: 'http://localhost' });
  setGlobals(dom.window);
  return dom.window.document.querySelector('[data-dashboard-shell]');
}

function click(win, el) {
  el.dispatchEvent(new win.MouseEvent('click', { bubbles: true }));
}

test('state helpers clamp sizes and sanitize stale persisted state', () => {
  assert.equal(shell.clampSize(100, 220, 420), 220);
  assert.equal(shell.clampSize(999, 220, 420), 420);
  assert.equal(shell.clampSize('320', 220, 420), 320);
  assert.equal(shell.clampSize('bad', 220, 420), null);

  const config = {
    surface: 'settings',
    regions: [
      { id: 'nav', resizable: true, min: 220, max: 420, defaultSize: 280 },
      { id: 'main', resizable: false },
    ],
    focusTargets: ['main'],
  };
  const state = shell.sanitizeShellState({
    regions: {
      nav: { collapsed: true, size: 999 },
      main: { collapsed: false, size: 300 },
      ghost: { collapsed: true, size: 1 },
    },
    focus: 'ghost',
  }, config);
  assert.equal(state.regions.nav.collapsed, true);
  assert.equal(state.regions.nav.size, 420);
  assert.equal(state.regions.main.size, null);
  assert.equal(state.regions.ghost, undefined);
  assert.equal(state.focus, null);

  const withInvalidSize = shell.sanitizeShellState({
    regions: { nav: { collapsed: false, size: 'bad' } },
  }, config);
  assert.equal(withInvalidSize.regions.nav.size, 280);
});

test('storage key is versioned, surface-scoped, and falls back to anonymous viewer', () => {
  assert.equal(
    shell.shellStorageKey({ surface: 'settings', version: 2, module: 'admin' }),
    'go-dashboard:shell:v2:settings:module:admin:viewer:anonymous',
  );
  assert.equal(
    shell.shellStorageKey({ surface: 'settings', viewer: 'u1' }),
    'go-dashboard:shell:v1:settings:viewer:u1',
  );
});

test('safe storage falls back to memory when backing storage throws', () => {
  const safe = shell.createSafeStorage({
    getItem() { throw new Error('blocked'); },
    setItem() { throw new Error('blocked'); },
    removeItem() { throw new Error('blocked'); },
  });
  safe.setItem('k', 'v');
  assert.equal(safe.getItem('k'), 'v');
  safe.removeItem('k');
  assert.equal(safe.getItem('k'), null);
});

test('buildShellConfig reads declarative shell attributes', () => {
  const root = setup();
  const config = shell.buildShellConfig(root, { storage: memStorage(), module: 'admin' });
  assert.equal(config.surface, 'settings');
  assert.equal(config.namespace, 'go-dashboard:shell');
  assert.equal(config.module, 'admin');
  assert.equal(config.regions.length, 2);
  assert.equal(config.regions[0].id, 'nav');
  assert.equal(config.regions[0].resizable, true);
  assert.equal(config.regions[0].min, 220);
  assert.deepEqual(config.focusTargets, ['main']);
});

test('controller restores clamped state and applies ARIA attributes', () => {
  const root = setup();
  const storage = memStorage({
    'go-dashboard:shell:v1:settings:viewer:anonymous': JSON.stringify({
      regions: { nav: { collapsed: true, size: 999 } },
      focus: 'main',
    }),
  });
  const controller = shell.initShell(root, { storage });
  const nav = root.querySelector('[data-shell-region="nav"]');
  const toggle = root.querySelector('[data-shell-toggle="nav"]');
  const focus = root.querySelector('[data-shell-focus-toggle="main"]');
  const handle = root.querySelector('[data-shell-resize="nav"]');
  const main = root.querySelector('[data-shell-region="main"]');

  assert.equal(nav.getAttribute('data-collapsed'), 'true');
  assert.equal(nav.style.width, '');
  assert.equal(toggle.getAttribute('aria-expanded'), 'false');
  assert.equal(root.getAttribute('data-shell-focus'), 'main');
  assert.equal(focus.getAttribute('aria-pressed'), 'true');
  assert.equal(main.getAttribute('data-shell-focus-active'), 'true');
  assert.equal(handle.getAttribute('aria-valuenow'), '420');
  controller.destroy();
});

test('collapse and focus controls update DOM and persistence', () => {
  const root = setup();
  const storage = memStorage();
  const controller = shell.initShell(root, { storage });
  const nav = root.querySelector('[data-shell-region="nav"]');
  const toggle = root.querySelector('[data-shell-toggle="nav"]');
  const focus = root.querySelector('[data-shell-focus-toggle="main"]');

  click(window, toggle);
  assert.equal(nav.getAttribute('data-collapsed'), 'true');
  assert.equal(toggle.getAttribute('aria-expanded'), 'false');
  click(window, focus);
  assert.equal(root.getAttribute('data-shell-focus'), 'main');
  assert.equal(focus.getAttribute('aria-pressed'), 'true');

  const persisted = JSON.parse(storage.getItem('go-dashboard:shell:v1:settings:viewer:anonymous'));
  assert.equal(persisted.regions.nav.collapsed, true);
  assert.equal(persisted.focus, 'main');
  controller.destroy();
});

test('keyboard resize adjusts separator-owned region within bounds', () => {
  const root = setup();
  const storage = memStorage();
  const controller = shell.initShell(root, { storage });
  const nav = root.querySelector('[data-shell-region="nav"]');
  const handle = root.querySelector('[data-shell-resize="nav"]');

  handle.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
  assert.equal(nav.style.width, '296px');
  assert.equal(handle.getAttribute('aria-valuenow'), '296');

  handle.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'Home', bubbles: true }));
  assert.equal(nav.style.width, '220px');

  handle.dispatchEvent(new window.KeyboardEvent('keydown', { key: 'End', bubbles: true }));
  assert.equal(nav.style.width, '420px');
  controller.destroy();
});

test('initShell is idempotent and initShells skips roots without regions', () => {
  const dom = new JSDOM(`<!doctype html><html><body>
    <section data-dashboard-shell data-dashboard-shell-surface="empty"></section>
    ${markup()}
  </body></html>`, { url: 'http://localhost' });
  setGlobals(dom.window);
  const controllers = shell.initShells(dom.window.document, { storage: memStorage() });
  assert.equal(controllers.length, 1);
  assert.equal(shell.initShell(dom.window.document.querySelector('[data-dashboard-shell-surface="settings"]')), null);
  controllers.forEach((controller) => controller.destroy());
});
