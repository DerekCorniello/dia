// time-tracker - a work-session logger that opens in its own window
// (ui.type=window). The browser-side panel is panel/panel.js; this
// entry is loaded in goja by the host but does nothing, because a
// window plugin's data lives fully on the client side. Panel state
// (sessions) is persisted in localStorage, not through the bridge.
module.exports = {
  getData: function () {
    return { hint: "see panel/panel.js" };
  }
};