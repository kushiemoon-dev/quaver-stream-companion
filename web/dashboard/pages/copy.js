(function () {
  // Resolves when text is on the clipboard; rejects when the browser has no clipboard access.
  function copy(text) {
    if (!navigator.clipboard || !navigator.clipboard.writeText) return Promise.reject(new Error('clipboard unavailable'));
    return navigator.clipboard.writeText(text);
  }

  window.qscCopy = { copy: copy };
})();
