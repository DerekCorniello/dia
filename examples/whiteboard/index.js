// whiteboard - a free-draw surface. The host renders this
// plugin's panel in its own OS-level window (ui.type=window).
// The browser-side panel is panel/panel.js; this entry is
// loaded in goja by the host and can be used for any
// "headless" data the panel needs (none for v1).
module.exports = {
  getData: function () {
    return { hint: "see panel/panel.js" };
  }
};
