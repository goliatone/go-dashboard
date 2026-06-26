(function (global) {
  'use strict';

  var SHELL_VERSION = 1;
  var STORAGE_NAMESPACE = 'go-dashboard:shell';
  var RESIZE_STEP = 16;

  function finiteNumber(value) {
    if (value === null || typeof value === 'undefined' || value === '') return null;
    var n = typeof value === 'number' ? value : Number(value);
    return Number.isFinite(n) ? n : null;
  }

  function clampSize(value, min, max) {
    var n = finiteNumber(value);
    if (n === null) return null;
    var lo = finiteNumber(min);
    var hi = finiteNumber(max);
    if (lo === null) lo = 0;
    if (hi === null) hi = Number.MAX_SAFE_INTEGER;
    if (hi < lo) {
      var tmp = lo;
      lo = hi;
      hi = tmp;
    }
    return Math.min(hi, Math.max(lo, n));
  }

  function shellStorageKey(config) {
    var namespace = config.namespace || STORAGE_NAMESPACE;
    var version = config.version || SHELL_VERSION;
    var surface = config.surface || 'dashboard';
    var parts = [namespace, 'v' + version, surface];
    if (config.module) parts.push('module', config.module);
    parts.push('viewer', config.viewer || 'anonymous');
    return parts.join(':');
  }

  function defaultShellState(config) {
    var regions = {};
    (config.regions || []).forEach(function (region) {
      regions[region.id] = {
        collapsed: region.defaultCollapsed === true,
        size: region.resizable ? clampSize(region.defaultSize, region.min, region.max) : null,
      };
    });
    return { regions: regions, focus: null };
  }

  function sanitizeShellState(raw, config) {
    var base = defaultShellState(config);
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return base;
    var rawRegions = raw.regions && typeof raw.regions === 'object' ? raw.regions : {};
    (config.regions || []).forEach(function (region) {
      var stored = rawRegions[region.id];
      if (!stored || typeof stored !== 'object') return;
      if (typeof stored.collapsed === 'boolean') {
        base.regions[region.id].collapsed = stored.collapsed;
      }
      if (region.resizable) {
        var size = clampSize(stored.size, region.min, region.max);
        if (size !== null) base.regions[region.id].size = size;
      } else {
        base.regions[region.id].size = null;
      }
    });
    var focusTargets = config.focusTargets || [];
    if (typeof raw.focus === 'string' && focusTargets.indexOf(raw.focus) >= 0) {
      base.focus = raw.focus;
    }
    return base;
  }

  function createSafeStorage(preferred) {
    var backing = preferred === undefined ? null : preferred;
    if (preferred === undefined) {
      try {
        backing = global.localStorage || null;
      } catch (error) {
        backing = null;
      }
    }
    var memory = new Map();
    return {
      getItem: function (key) {
        if (backing) {
          try {
            return backing.getItem(key);
          } catch (error) {}
        }
        return memory.has(key) ? memory.get(key) : null;
      },
      setItem: function (key, value) {
        if (backing) {
          try {
            backing.setItem(key, String(value));
            return;
          } catch (error) {}
        }
        memory.set(key, String(value));
      },
      removeItem: function (key) {
        if (backing) {
          try {
            backing.removeItem(key);
          } catch (error) {}
        }
        memory.delete(key);
      },
    };
  }

  function numericAttr(el, name) {
    var raw = el.getAttribute(name);
    if (raw === null || String(raw).trim() === '') return undefined;
    var n = Number(raw);
    return Number.isFinite(n) ? n : undefined;
  }

  function buildShellConfig(root, options) {
    options = options || {};
    var regions = [];
    var seen = new Set();
    root.querySelectorAll('[data-shell-region]').forEach(function (el) {
      var id = el.getAttribute('data-shell-region');
      if (!id || seen.has(id)) return;
      seen.add(id);
      regions.push({
        id: id,
        resizable: el.hasAttribute('data-shell-resizable'),
        edge: el.getAttribute('data-shell-edge') || 'trailing',
        min: numericAttr(el, 'data-shell-min'),
        max: numericAttr(el, 'data-shell-max'),
        defaultSize: numericAttr(el, 'data-shell-default-size'),
        defaultCollapsed: el.getAttribute('data-collapsed') === 'true',
      });
    });
    var focusTargets = [];
    root.querySelectorAll('[data-shell-focus-toggle]').forEach(function (el) {
      var id = el.getAttribute('data-shell-focus-toggle');
      if (id && focusTargets.indexOf(id) < 0) focusTargets.push(id);
    });
    var version = numericAttr(root, 'data-dashboard-shell-version') || SHELL_VERSION;
    return {
      namespace: root.getAttribute('data-dashboard-shell-namespace') || options.namespace || STORAGE_NAMESPACE,
      version: version,
      surface: root.getAttribute('data-dashboard-shell-surface') || 'dashboard',
      viewer: root.getAttribute('data-dashboard-shell-viewer') || options.viewer || '',
      module: root.getAttribute('data-dashboard-shell-module') || options.module || '',
      regions: regions,
      focusTargets: focusTargets,
      storage: Object.prototype.hasOwnProperty.call(options, 'storage') ? options.storage : undefined,
      onChange: options.onChange,
    };
  }

  function closestInRoot(target, selector, root) {
    var el = target && target.closest ? target.closest(selector) : null;
    return el && root.contains(el) ? el : null;
  }

  function ShellController(root, config) {
    this.root = root;
    this.config = config;
    this.regionDefs = new Map((config.regions || []).map(function (region) { return [region.id, region]; }));
    this.storage = createSafeStorage(config.storage);
    this.storageKey = shellStorageKey(config);
    this.state = this.restore();
    this.cleanups = [];
    this.dragCleanup = null;
  }

  ShellController.prototype.restore = function () {
    var raw = null;
    var stored = this.storage.getItem(this.storageKey);
    if (stored) {
      try {
        raw = JSON.parse(stored);
      } catch (error) {
        raw = null;
      }
    }
    return sanitizeShellState(raw, this.config);
  };

  ShellController.prototype.persist = function () {
    this.storage.setItem(this.storageKey, JSON.stringify(this.state));
  };

  ShellController.prototype.emitChange = function () {
    if (typeof this.config.onChange === 'function') {
      this.config.onChange(this.getState());
    }
  };

  ShellController.prototype.getState = function () {
    var regions = {};
    Object.keys(this.state.regions).forEach(function (id) {
      regions[id] = {
        collapsed: this.state.regions[id].collapsed,
        size: this.state.regions[id].size,
      };
    }, this);
    return { regions: regions, focus: this.state.focus };
  };

  ShellController.prototype.init = function () {
    this.apply();
    this.bindControls();
    return this;
  };

  ShellController.prototype.apply = function () {
    Object.keys(this.state.regions).forEach(this.applyRegion, this);
    this.applyFocus();
  };

  ShellController.prototype.applyRegion = function (id) {
    var state = this.state.regions[id];
    var def = this.regionDefs.get(id);
    if (!state || !def) return;
    var el = this.root.querySelector('[data-shell-region="' + id + '"]');
    if (el) {
      el.setAttribute('data-collapsed', state.collapsed ? 'true' : 'false');
      if (def.resizable && state.size !== null && !state.collapsed) {
        el.style.flexBasis = state.size + 'px';
        el.style.width = state.size + 'px';
      } else {
        el.style.removeProperty('flex-basis');
        el.style.removeProperty('width');
      }
    }
    this.root.querySelectorAll('[data-shell-toggle="' + id + '"]').forEach(function (toggle) {
      toggle.setAttribute('aria-expanded', state.collapsed ? 'false' : 'true');
      toggle.setAttribute('data-shell-collapsed', state.collapsed ? 'true' : 'false');
    });
    this.root.querySelectorAll('[data-shell-resize="' + id + '"]').forEach(function (handle) {
      if (state.size !== null) handle.setAttribute('aria-valuenow', String(state.size));
    });
  };

  ShellController.prototype.applyFocus = function () {
    if (this.state.focus) {
      this.root.setAttribute('data-shell-focus', this.state.focus);
    } else {
      this.root.removeAttribute('data-shell-focus');
    }
    this.root.querySelectorAll('[data-shell-focus-toggle]').forEach(function (toggle) {
      var target = toggle.getAttribute('data-shell-focus-toggle') || '';
      toggle.setAttribute('aria-pressed', this.state.focus === target ? 'true' : 'false');
    }, this);
    this.root.querySelectorAll('[data-shell-focus-exit]').forEach(function (button) {
      button.setAttribute('aria-pressed', this.state.focus ? 'true' : 'false');
    }, this);
    this.root.querySelectorAll('[data-shell-focus-target]').forEach(function (region) {
      var target = region.getAttribute('data-shell-focus-target') || '';
      region.setAttribute('data-shell-focus-active', this.state.focus === target ? 'true' : 'false');
    }, this);
  };

  ShellController.prototype.setRegionCollapsed = function (id, collapsed, opts) {
    var state = this.state.regions[id];
    if (!state || state.collapsed === collapsed) return;
    state.collapsed = collapsed;
    this.applyRegion(id);
    if (!opts || opts.persist !== false) this.persist();
    this.emitChange();
  };

  ShellController.prototype.toggleRegion = function (id) {
    var state = this.state.regions[id];
    if (!state) return;
    this.setRegionCollapsed(id, !state.collapsed);
  };

  ShellController.prototype.setRegionSize = function (id, size, opts) {
    var def = this.regionDefs.get(id);
    var state = this.state.regions[id];
    if (!def || !def.resizable || !state) return;
    var next = clampSize(size, def.min, def.max);
    if (next === null) return;
    state.size = next;
    this.applyRegion(id);
    if (!opts || opts.persist !== false) this.persist();
    this.emitChange();
  };

  ShellController.prototype.setFocus = function (id, opts) {
    var focusTargets = this.config.focusTargets || [];
    var next = id && focusTargets.indexOf(id) >= 0 ? id : null;
    if (this.state.focus === next) return;
    this.state.focus = next;
    this.applyFocus();
    if (!opts || opts.persist !== false) this.persist();
    this.emitChange();
  };

  ShellController.prototype.toggleFocus = function (id) {
    this.setFocus(this.state.focus === id ? null : id);
  };

  ShellController.prototype.bindControls = function () {
    var self = this;
    function onClick(event) {
      var toggle = closestInRoot(event.target, '[data-shell-toggle]', self.root);
      if (toggle) {
        event.preventDefault();
        self.toggleRegion(toggle.getAttribute('data-shell-toggle'));
        return;
      }
      var exit = closestInRoot(event.target, '[data-shell-focus-exit]', self.root);
      if (exit) {
        event.preventDefault();
        self.setFocus(null);
        return;
      }
      var focus = closestInRoot(event.target, '[data-shell-focus-toggle]', self.root);
      if (focus) {
        event.preventDefault();
        self.toggleFocus(focus.getAttribute('data-shell-focus-toggle'));
      }
    }
    function onMouseDown(event) {
      var handle = closestInRoot(event.target, '[data-shell-resize]', self.root);
      if (handle) self.beginResize(handle.getAttribute('data-shell-resize'), event);
    }
    function onKeyDown(event) {
      var handle = closestInRoot(event.target, '[data-shell-resize]', self.root);
      if (!handle) return;
      var id = handle.getAttribute('data-shell-resize');
      var def = self.regionDefs.get(id);
      var state = self.state.regions[id];
      if (!def || !state || state.collapsed) return;
      var current = state.size === null ? def.defaultSize : state.size;
      var next = current;
      if (event.key === 'ArrowLeft') next = current + (def.edge === 'leading' ? RESIZE_STEP : -RESIZE_STEP);
      else if (event.key === 'ArrowRight') next = current + (def.edge === 'leading' ? -RESIZE_STEP : RESIZE_STEP);
      else if (event.key === 'Home') next = def.min;
      else if (event.key === 'End') next = def.max;
      else return;
      event.preventDefault();
      self.setRegionSize(id, next);
    }
    this.root.addEventListener('click', onClick);
    this.root.addEventListener('mousedown', onMouseDown);
    this.root.addEventListener('keydown', onKeyDown);
    this.cleanups.push(function () {
      self.root.removeEventListener('click', onClick);
      self.root.removeEventListener('mousedown', onMouseDown);
      self.root.removeEventListener('keydown', onKeyDown);
    });
  };

  ShellController.prototype.beginResize = function (id, event) {
    var def = this.regionDefs.get(id);
    var state = this.state.regions[id];
    var el = this.root.querySelector('[data-shell-region="' + id + '"]');
    if (!def || !def.resizable || !state || !el || state.collapsed) return;
    event.preventDefault();
    this.endResize();
    var self = this;
    var startX = event.clientX;
    var startSize = state.size === null ? el.getBoundingClientRect().width : state.size;
    function onMove(move) {
      var dx = move.clientX - startX;
      var delta = def.edge === 'leading' ? -dx : dx;
      self.setRegionSize(id, startSize + delta, { persist: false });
    }
    function onUp() {
      self.endResize();
      self.persist();
    }
    var ownerDoc = el.ownerDocument || global.document;
    ownerDoc.addEventListener('mousemove', onMove);
    ownerDoc.addEventListener('mouseup', onUp);
    this.root.setAttribute('data-shell-resizing', id);
    this.dragCleanup = function () {
      ownerDoc.removeEventListener('mousemove', onMove);
      ownerDoc.removeEventListener('mouseup', onUp);
      self.root.removeAttribute('data-shell-resizing');
      self.dragCleanup = null;
    };
  };

  ShellController.prototype.endResize = function () {
    if (this.dragCleanup) this.dragCleanup();
  };

  ShellController.prototype.destroy = function () {
    this.endResize();
    this.cleanups.forEach(function (cleanup) { cleanup(); });
    this.cleanups.length = 0;
  };

  function initShell(root, options) {
    if (!root || root.getAttribute('data-dashboard-shell-init') === 'true') return null;
    var config = buildShellConfig(root, options);
    if (config.regions.length === 0) return null;
    var controller = new ShellController(root, config).init();
    root.setAttribute('data-dashboard-shell-init', 'true');
    return controller;
  }

  function initShells(scope, options) {
    scope = scope || global.document;
    if (!scope || !scope.querySelectorAll) return [];
    var controllers = [];
    scope.querySelectorAll('[data-dashboard-shell]').forEach(function (root) {
      var controller = initShell(root, options);
      if (controller) controllers.push(controller);
    });
    return controllers;
  }

  var api = {
    SHELL_VERSION: SHELL_VERSION,
    STORAGE_NAMESPACE: STORAGE_NAMESPACE,
    clampSize: clampSize,
    shellStorageKey: shellStorageKey,
    defaultShellState: defaultShellState,
    sanitizeShellState: sanitizeShellState,
    createSafeStorage: createSafeStorage,
    buildShellConfig: buildShellConfig,
    ShellController: ShellController,
    initShell: initShell,
    initShells: initShells,
  };

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
  global.DashboardShell = api;

  if (global.document) {
    if (global.document.readyState === 'loading') {
      global.document.addEventListener('DOMContentLoaded', function () { initShells(global.document); });
    } else {
      initShells(global.document);
    }
  }
})(typeof window !== 'undefined' ? window : globalThis);
