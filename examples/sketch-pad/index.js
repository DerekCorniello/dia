// sketch-pad - a minimal example of the canvas panel type.
// getData() still runs on load and refresh, but its value is not
// drawn: strokes come from the user drawing on the canvas. The
// color/width fields it returns set the pen used for the *next*
// stroke, sourced here from config_schema so they are configurable
// per workspace instead of hardcoded.
// onAction can replace what is on the canvas by returning a new
// strokes array; Undo uses that to drop the most recent stroke.
module.exports = {
  getData: function () {
    var cfg = dia.getConfig();
    return {
      color: cfg.color || "#1b2636",
      width: cfg.width || 3
    };
  },
  onAction: function (id, ctx) {
    if (id === "undo") {
      return { strokes: ctx.strokes.slice(0, -1) };
    }
  }
};
