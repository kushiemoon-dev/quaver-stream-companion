// Dashboard shell: routing, navigation state, skip link, start-up language and the helpers pages build on.
'use strict';
const assert = require('node:assert/strict');
const { test, run } = require('../../testsupport/run.js');
const H = require('./helpers.js');

const nav = (win) => win.$$('.nav-item').map((el) => ({ page: el.getAttribute('data-page'), label: el.textContent, current: el.getAttribute('aria-current') }));
const current = (win) => nav(win).filter((n) => n.current).map((n) => n.page);

test('starts on the status page and fixes the empty address', async () => {
  const win = await H.open();
  assert.equal(win.location.hash, '#/status');
  assert.equal(win.document.title, 'Status - QuaverStreamCompanion');
  assert.deepEqual(current(win), ['status']);
  assert.equal(win.$('#main h1').textContent, 'Status');
  assert.equal(win.document.documentElement.lang, 'en');
});

test('the navigation lists every page, labelled, with Twitch shown as coming soon and not a link', async () => {
  const win = await H.open();
  assert.deepEqual(nav(win).map((n) => n.page), ['status', 'overlays', 'obs', 'history', 'settings', 'twitch']);
  assert.equal(win.$('#nav').getAttribute('aria-label'), 'Main navigation');
  const links = win.$$('a.nav-item');
  assert.deepEqual(links.map((a) => a.getAttribute('href')), ['#/status', '#/overlays', '#/obs', '#/history', '#/settings']);
  assert.deepEqual(links.map((a) => a.getAttribute('aria-label')), ['Status', 'Overlays', 'OBS', 'History', 'Settings']);
  const twitch = win.$('.nav-item[data-page="twitch"]');
  assert.equal(twitch.localName, 'span');
  assert.equal(twitch.getAttribute('aria-disabled'), 'true');
  assert.match(twitch.textContent, /Coming soon/);
  assert.equal(win.$('.nav-icon').getAttribute('aria-hidden'), 'true');
});

test('a hash change shows that page, moves the current marker, retitles and focuses the main area', async () => {
  const win = await H.open();
  win.location.hash = '#/history';
  await win.flush();
  assert.equal(win.document.title, 'History - QuaverStreamCompanion');
  assert.deepEqual(current(win), ['history']);
  assert.equal(win.document.activeElement, win.$('#main'));
  assert.equal(win.$('#main h1').textContent, 'History');
  win.location.hash = '#/settings';
  await win.flush();
  assert.deepEqual(current(win), ['settings']);
  assert.equal(win.document.title, 'Settings - QuaverStreamCompanion');
});

test('leaving a page unmounts it: its event stream closes and is not reopened', async () => {
  const win = await H.open();
  const es = H.stream(win);
  assert.equal(es.closed, false);
  win.qsc.registerPage('quiet', { title: 'nav.obs', mount() {} });
  win.location.hash = '#/quiet';
  await win.flush();
  assert.equal(es.closed, true);
  await win.advance(20000);
  assert.equal(win.eventSources.length, 1);
});

test('unknown pages and the unreleased Twitch page fall back to status and rewrite the address', async () => {
  for (const hash of ['#/nope', '#/twitch', '#/', '#garbage']) {
    const win = await H.open({ hash });
    assert.equal(win.location.hash, '#/status', hash);
    assert.deepEqual(current(win), ['status'], hash);
    assert.equal(win.document.title, 'Status - QuaverStreamCompanion', hash);
  }
});

test('a known page with a sub path keeps the address and shows the page', async () => {
  const win = await H.open({ hash: '#/history/extra' });
  assert.equal(win.location.hash, '#/history/extra');
  assert.deepEqual(current(win), ['history']);
});

test('the skip link moves focus to the main area instead of changing the address', async () => {
  const win = await H.open();
  const skip = win.$('#skip');
  assert.equal(skip.getAttribute('href'), '#main');
  assert.equal(skip.textContent, 'Skip to content');
  const ev = new win.Event('click');
  skip.dispatchEvent(ev);
  assert.equal(ev.defaultPrevented, true);
  assert.equal(win.document.activeElement, win.$('#main'));
  assert.equal(win.location.hash, '#/status');
});

test('the configured language labels the navigation, the skip link and the page title', async () => {
  const win = await H.open({ routes: { '/api/config': () => H.json({ config: { language: 'fr' } }) } });
  const fr = H.catalog('fr');
  assert.equal(win.document.documentElement.lang, 'fr');
  assert.equal(win.$('#skip').textContent, fr['skip.link']);
  assert.equal(win.$('.nav-item[data-page="status"]').textContent, fr['nav.status']);
  assert.equal(win.document.title, fr['nav.status'] + ' - QuaverStreamCompanion');
});

test('without a configured language the browser language decides, and a failed config request is not fatal', async () => {
  const win = await H.open({ language: 'de-AT', routes: { '/api/config': () => new Error('down') } });
  assert.equal(win.document.documentElement.lang, 'de');
  assert.equal(win.$('#skip').textContent, H.catalog('de')['skip.link']);
  assert.equal(win.document.title, H.catalog('de')['nav.status'] + ' - QuaverStreamCompanion');
});

test('a missing translation file leaves the English texts in place', async () => {
  const win = await H.open({ routes: { '/api/config': () => H.json({ config: { language: 'fr' } }), 'i18n/fr.json': () => H.json({}, 404) } });
  assert.equal(win.$('#skip').textContent, 'Skip to content');
  assert.notEqual(win.document.documentElement.lang, 'fr');
});

test('relabel redraws the navigation in a new language and keeps the current marker and the title', async () => {
  const win = await H.open({ hash: '#/history' });
  await win.qscI18n.load('de');
  win.qsc.relabel();
  const de = H.catalog('de');
  assert.deepEqual(current(win), ['history']);
  assert.equal(win.$('.nav-item[data-page="history"]').textContent, de['nav.history']);
  assert.equal(win.$('#skip').textContent, de['skip.link']);
  assert.equal(win.document.title, de['nav.history'] + ' - QuaverStreamCompanion');
});

test('relabel repaints the offline banner in the new language', async () => {
  const win = await H.open();
  win.qscOffline.report(false);
  assert.equal(win.$('#offline').textContent, H.en['offline.banner']);
  await win.qscI18n.load('fr');
  win.qsc.relabel();
  assert.equal(win.$('#offline').textContent, H.catalog('fr')['offline.banner']);
});

test('registerPage adds a page that navigate reaches, and navigating to the shown page renders it again', async () => {
  const win = await H.open();
  const log = [];
  win.qsc.registerPage('extra', { title: 'nav.obs', mount: (root) => { log.push('mount'); root.textContent = 'extra page'; }, unmount: () => log.push('unmount') });
  win.qsc.navigate('#/extra');
  await win.flush();
  assert.equal(win.$('#main').textContent, 'extra page');
  assert.equal(win.document.title, 'OBS - QuaverStreamCompanion');
  win.qsc.navigate('#/extra');
  assert.deepEqual(log, ['mount', 'unmount', 'mount']);
  win.qsc.rerender();
  assert.deepEqual(log, ['mount', 'unmount', 'mount', 'unmount', 'mount']);
});

test('h builds elements: attributes, handlers, boolean flags, nested lists and plain text children', async () => {
  const win = await H.open();
  const { h } = win.qsc;
  let clicked = 0;
  const el = h('button', { class: 'a', hidden: true, disabled: false, title: null, 'data-x': 0, onclick: () => { clicked++; } }, 'one', [h('b', null, 'two'), null, [3]], false);
  assert.equal(el.getAttribute('class'), 'a');
  assert.equal(el.getAttribute('hidden'), '');
  assert.equal(el.hasAttribute('disabled'), false);
  assert.equal(el.hasAttribute('title'), false);
  assert.equal(el.getAttribute('data-x'), '0');
  assert.equal(el.textContent, 'onetwo3');
  el.click();
  assert.equal(clicked, 1);
});

test('h never turns text into markup', async () => {
  const win = await H.open();
  const el = win.qsc.h('p', null, '<img src=x onerror=alert(1)>');
  assert.equal(el.children.length, 0);
  assert.equal(el.textContent, '<img src=x onerror=alert(1)>');
});

test('pathNodes allows a line break after every slash and backslash and keeps the text intact', async () => {
  const win = await H.open();
  const bs = String.fromCharCode(92);
  const nodes = win.qsc.pathNodes('C:' + bs + 'Games/Quaver');
  assert.equal(nodes.map((n) => n.textContent).join(''), 'C:' + bs + 'Games/Quaver');
  assert.deepEqual(Array.from(nodes.filter((n) => n.nodeType === 1), (n) => n.localName), ['wbr', 'wbr']);
  assert.equal(win.qsc.pathNodes('').length, 0);
});

run();
