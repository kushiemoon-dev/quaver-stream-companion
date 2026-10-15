(function () {
  function isObject(v) {
    return v !== null && typeof v === 'object' && !Array.isArray(v);
  }

  function isScore(r) {
    return isObject(r) && isObject(r.score) && isObject(r.map);
  }

  function initial() {
    return { game: 'unknown', map: null, last_score: null, unsaved: false };
  }

  // A play.ended can arrive after the next play has started; it only counts for the map now shown.
  function endedFits(state, d) {
    return isObject(d.map) && isObject(state.map) && typeof d.map.md5 === 'string' && d.map.md5 === state.map.md5;
  }

  function reduce(state, name, d) {
    if (!isObject(d)) throw new Error('bad payload');
    if (name === 'snapshot') {
      return { game: typeof d.game === 'string' ? d.game : 'unknown', map: isObject(d.map) ? d.map : null, last_score: isScore(d.last_score) ? d.last_score : null, unsaved: false };
    }
    if (name === 'game.state') {
      if (typeof d.state !== 'string') throw new Error('bad payload');
      return Object.assign({}, state, { game: d.state });
    }
    if (name === 'map.started') return Object.assign({}, state, { map: d, unsaved: false });
    if (name === 'play.ended') return d.saved === false && endedFits(state, d) ? Object.assign({}, state, { unsaved: true }) : state;
    if (!isScore(d)) throw new Error('bad payload');
    return Object.assign({}, state, { last_score: d, unsaved: false });
  }

  // The note sits under the last play card, so it only makes sense when a previous play is shown.
  function showUnsaved(state) {
    return state.unsaved === true && isScore(state.last_score);
  }

  const api = { initial: initial, reduce: reduce, showUnsaved: showUnsaved };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else window.qscStatusModel = api;
})();
