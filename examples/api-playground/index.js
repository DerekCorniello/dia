// api-playground - a mini Postman that opens in its own window
// (ui.type=window). The browser-side panel is panel/panel.js; this
// entry is loaded in goja by the host but does nothing, because a
// window plugin's data lives fully on the client side. Requests are
// sent through window.dia.call("fetch", ...), which requires the
// mutating "fetch" capability -- approve it when you enable this
// plugin.
module.exports = {
  getData: function () {
    return { hint: "see panel/panel.js" };
  }
};