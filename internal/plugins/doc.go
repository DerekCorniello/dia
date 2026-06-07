// Package plugins implements the dia plugin system. A plugin is a
// folder under the user's dia state directory that contains a
// plugin.json manifest and one or more JavaScript files executed
// in-process by a goja interpreter. Plugins can call back into dia
// through a capability-gated bridge object, but cannot reach the host
// DOM, the network, or the filesystem directly.
package plugins

import "regexp"

const (
	pluginsDirName   = "plugins"
	manifestFile     = "plugin.json"
	defaultEntry     = "index.js"
	defaultPanelDir  = "panel"
	DefaultPanelHTML = "panel/index.html"
	DefaultPanelJS   = "panel/panel.js"
	DefaultPanelCSS  = "panel/styles.css"
	nameMaxLen       = 60
	descMaxLen       = 200
	longDescMaxLen   = 2000
	authorMaxLen     = 60
	versionMaxLen    = 32
)

var idPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])?$`)

func validID(s string) bool {
	if len(s) < 3 {
		return false
	}
	if !idPattern.MatchString(s) {
		return false
	}
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '-' && s[i+1] == '-' {
			return false
		}
	}
	return true
}
