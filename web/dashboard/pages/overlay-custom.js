(function () {
  const qsc = window.qsc;
  const t = qsc.t;
  const h = qsc.h;
  const FRAME_EXTRA = 20;
  const FRAME_MAX = 480;

  function name(o) {
    return o.custom ? o.name : t(o.nameKey);
  }

  function desc(o) {
    return o.custom ? o.description : t(o.descKey);
  }

  function frameClass(o) {
    return o.custom ? 'preview-frame' : 'preview-frame size-' + o.id;
  }

  // A custom overlay has no stylesheet class, so its preview height follows its declared height.
  function frameHeight(frame, o) {
    frame.style.height = o.custom ? Math.min(o.height + FRAME_EXTRA, FRAME_MAX) + 'px' : '';
  }

  function frameTitle(frame, o, demo) {
    frame.setAttribute('title', t(demo !== false && !o.custom ? 'overlays.preview_title' : 'overlays.preview_title_live', { name: name(o) }));
  }

  function dirHelp(dir) {
    return dir ? h('p', { class: 'muted hint' }, t('overlays.custom_dir_help'), ' ', h('code', null, qsc.pathNodes(dir))) : null;
  }

  window.qscOverlayCustom = { name: name, desc: desc, frameClass: frameClass, frameHeight: frameHeight, frameTitle: frameTitle, dirHelp: dirHelp };
})();
