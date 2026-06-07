// hello-plugin - a minimal example dia plugin.
// The host invokes getData() before rendering the panel and again
// when the user clicks the refresh button (ui.refreshable=true).
// Capabilities declared in plugin.json are enforced; calling
// dia.startWorkspace() without "workspaces:start" in the grant list
// will throw.
module.exports = {
  getData: function () {
    var workspaces = dia.listWorkspaces();
    return workspaces.map(function (w) {
      return {
        id: w.name,
        label: w.name,
        path: w.path || ""
      };
    });
  }
};
