(function () {
  const SVG_NS = 'http://www.w3.org/2000/svg';
  const t = window.qscI18n.t;
  const pages = Object.create(null);
  let current = null;

  const NAV = [
    { id: 'status', icon: 'M3 12h4l3-8 4 16 3-8h4' },
    { id: 'overlays', icon: 'M12 3l9 5-9 5-9-5 9-5z M3 13l9 5 9-5' },
    { id: 'obs', icon: 'M4 5h16v11H4z M8 20h8 M12 16v4' },
    { id: 'history', icon: 'M3 12a9 9 0 1 0 3-6.7 M3 4v5h5 M12 7v5l3 2' },
    { id: 'settings', icon: 'M4 7h10 M18 7h2 M4 17h2 M10 17h10 M14 5v4 M6 15v4' },
    { id: 'twitch', icon: 'M4 4h16v12H9l-5 4z', disabled: true },
  ];

  function h(tag, attrs) {
    const el = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        const v = attrs[k];
        if (v === null || v === undefined || v === false) return;
        if (k.indexOf('on') === 0 && typeof v === 'function') el.addEventListener(k.slice(2), v);
        else el.setAttribute(k, v === true ? '' : v);
      });
    }
    for (let i = 2; i < arguments.length; i++) append(el, arguments[i]);
    return el;
  }

  function append(el, child) {
    if (child === null || child === undefined || child === false) return;
    if (Array.isArray(child)) child.forEach(function (c) { append(el, c); });
    else el.appendChild(child.nodeType ? child : document.createTextNode(String(child)));
  }

  function pathNodes(text) {
    const out = [];
    String(text).split(/([\\/])/).forEach(function (part) {
      if (part === '') return;
      out.push(document.createTextNode(part));
      if (part === '\\' || part === '/') out.push(document.createElement('wbr'));
    });
    return out;
  }

  function icon(d) {
    const svg = document.createElementNS(SVG_NS, 'svg');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('width', '18');
    svg.setAttribute('height', '18');
    svg.setAttribute('aria-hidden', 'true');
    svg.setAttribute('class', 'nav-icon');
    const path = document.createElementNS(SVG_NS, 'path');
    path.setAttribute('d', d);
    svg.appendChild(path);
    return svg;
  }

  function buildNav() {
    const nav = document.getElementById('nav');
    nav.setAttribute('aria-label', t('nav.label'));
    nav.replaceChildren();
    NAV.forEach(function (item) {
      if (item.disabled) {
        nav.appendChild(h('span', { class: 'nav-item is-disabled', 'aria-disabled': 'true', 'data-page': item.id },
          icon(item.icon), h('span', { class: 'nav-label' }, t('nav.' + item.id)),
          h('span', { class: 'pill pill-muted' }, t('nav.soon'))));
        return;
      }
      nav.appendChild(h('a', { class: 'nav-item', href: '#/' + item.id, 'aria-label': t('nav.' + item.id), 'data-page': item.id },
        icon(item.icon), h('span', { class: 'nav-label' }, t('nav.' + item.id))));
    });
  }

  function routeName() {
    const name = location.hash.replace(/^#\/?/, '').split('/')[0];
    return pages[name] ? name : 'status';
  }

  function labelChrome() {
    document.getElementById('skip').textContent = t('skip.link');
    window.qscOffline.repaint();
  }

  function render() {
    const name = routeName();
    if (location.hash !== '#/' + name && !pages[location.hash.replace(/^#\/?/, '').split('/')[0]]) history.replaceState(null, '', '#/' + name);
    const main = document.getElementById('main');
    if (current && current.page.unmount) current.page.unmount();
    main.replaceChildren();
    current = { name: name, page: pages[name] };
    document.querySelectorAll('.nav-item[data-page]').forEach(function (el) {
      if (el.getAttribute('data-page') === name) el.setAttribute('aria-current', 'page');
      else el.removeAttribute('aria-current');
    });
    document.title = t(current.page.title) + ' - QuaverStreamCompanion';
    current.page.mount(main);
  }

  function relabel() {
    buildNav();
    labelChrome();
    const name = current ? current.name : routeName();
    document.querySelectorAll('.nav-item[data-page]').forEach(function (el) {
      if (el.getAttribute('data-page') === name) el.setAttribute('aria-current', 'page');
    });
    if (current) document.title = t(current.page.title) + ' - QuaverStreamCompanion';
  }

  function registerPage(name, page) {
    pages[name] = page;
  }

  function navigate(hash) {
    if (location.hash === hash) render();
    else location.hash = hash;
  }

  async function start() {
    let language = '';
    try {
      const cfg = await window.qscApi.get('/api/config');
      language = (cfg && cfg.config && cfg.config.language) || '';
    } catch (e) {
      language = '';
    }
    await window.qscI18n.load(language);
    buildNav();
    labelChrome();
    window.qscOffline.start();
    document.getElementById('skip').addEventListener('click', function (ev) {
      ev.preventDefault();
      document.getElementById('main').focus();
    });
    window.addEventListener('hashchange', function () {
      render();
      document.getElementById('main').focus();
    });
    render();
  }

  window.qsc = { t: t, h: h, pathNodes: pathNodes, api: window.qscApi, navigate: navigate, relabel: relabel, rerender: render, registerPage: registerPage };
  window.addEventListener('DOMContentLoaded', start);
})();
