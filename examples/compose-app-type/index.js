// An app-type plugin. Unlike a panel plugin, the interesting export is
// resolveApp: dia calls it while starting a workspace and launches
// whatever it returns.
//
// resolveApp runs in a restricted runtime. The only dia.* calls
// available are getConfig() and pluginDir() -- no workspaces, no exec,
// no fetch. It must be a pure function of the app config it is handed,
// which is what keeps `dia start --dry-run` honest.

function composeBinary() {
  var cfg = dia.getConfig() || {};
  return cfg.binary || 'docker compose';
}

module.exports = {
  // Called once per app of a type this plugin claims. `app` is the
  // workspace entry exactly as written in YAML: type, cmd, args, cwd,
  // env, url.
  //
  // Return { cmd, args?, cwd?, env? } to launch something, or
  // { url } to open a URL. Exactly one of cmd/url.
  resolveApp: function (app) {
    var args = app.args || [];

    if (app.type === 'compose:down') {
      return {
        cmd: composeBinary(),
        args: ['down'].concat(args),
        cwd: app.cwd,
      };
    }

    // type: compose
    return {
      cmd: composeBinary(),
      args: ['up', '-d'].concat(args),
      cwd: app.cwd,
      env: app.env,
    };
  },

  // The panel is incidental here, but a manifest still needs a ui
  // block, and getData has to exist for the kv panel to render.
  getData: function () {
    return {
      binary: composeBinary(),
      provides: 'compose, compose:down',
    };
  },
};
