(function () {
  const ID_RE = /^[a-z0-9-]+$/;
  const HEX_RE = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
  const NAMES = {
    'neutral-light': 'Neutral Light',
    'neutral-dark': 'Neutral Dark',
    'catppuccin-mocha': 'Catppuccin Mocha',
    'catppuccin-latte': 'Catppuccin Latte',
    'tokyo-night': 'Tokyo Night',
    'dracula': 'Dracula',
    'nord': 'Nord',
    'gruvbox': 'Gruvbox',
    'rose-pine': 'Rose Pine',
    'pixel': 'Pixel',
  };
  const ORDER = Object.keys(NAMES);

  function displayName(id) {
    return Object.prototype.hasOwnProperty.call(NAMES, id) ? NAMES[id] : String(id);
  }

  function isHexColor(value) {
    return typeof value === 'string' && HEX_RE.test(value);
  }

  function safeColors(colors) {
    const c = colors && typeof colors === 'object' ? colors : {};
    return {
      bg: isHexColor(c.bg) ? c.bg : '',
      fg: isHexColor(c.fg) ? c.fg : '',
      accent: isHexColor(c.accent) ? c.accent : '',
    };
  }

  function rank(id) {
    const i = ORDER.indexOf(id);
    return i < 0 ? ORDER.length : i;
  }

  // Keeps usable entries once each, known themes first in their order, others alphabetically.
  function orderThemes(list) {
    const seen = {};
    const out = [];
    (Array.isArray(list) ? list : []).forEach(function (item) {
      if (!item || typeof item.id !== 'string' || !ID_RE.test(item.id) || seen[item.id]) return;
      seen[item.id] = true;
      out.push({ id: item.id, colors: safeColors(item.colors) });
    });
    return out.sort(function (a, b) {
      return rank(a.id) - rank(b.id) || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
    });
  }

  // A saved theme missing from the list stays selectable, as a custom entry without colors.
  function buildCards(list, selected, defaultTheme) {
    const themes = orderThemes(list);
    if (ID_RE.test(selected) && !themes.some(function (t) { return t.id === selected; })) {
      themes.push({ id: selected, colors: safeColors(null) });
    }
    return themes.map(function (t) {
      return {
        id: t.id,
        name: displayName(t.id),
        custom: rank(t.id) === ORDER.length,
        colors: t.colors,
        selected: t.id === selected,
        isDefault: t.id === defaultTheme,
      };
    });
  }

  const api = { displayName: displayName, isHexColor: isHexColor, orderThemes: orderThemes, buildCards: buildCards };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscThemeModel = api;
})();
