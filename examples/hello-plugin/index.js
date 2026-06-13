// hello-plugin - a minimal example dia plugin.
// The host invokes getData() before rendering the panel and again
// when the user clicks the refresh button (ui.refreshable=true).
// Capabilities declared in plugin.json are enforced; calling
// dia.startWorkspace() without "workspaces:start" in the grant list
// will throw.
// Config declared in config_schema is accessible via dia.getConfig().
module.exports = {
  getData: function () {
    var cfg = dia.getConfig();
    var workspaces = dia.listWorkspaces();
    if (cfg.show_all) {
      return workspaces.slice(0, cfg.max_items || 10).map(function (w) {
        return {
          id: w.name,
          label: w.name,
          path: w.path || ""
        };
      });
    }
    var recent = dia.listWorkspaces().slice(0, cfg.max_items || 5);
    return recent.map(function (w) {
      return {
        id: w.name,
        label: w.name,
        path: w.path || ""
      };
    });
  }
};
