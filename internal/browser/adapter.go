package browser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// adapter isolates everything browser-family-specific: where the user's
// real profile lives, how to launch against a chosen profile dir, and
// which subdirectories are pure cache and safe to skip when cloning.
type adapter interface {
	// seedProfile returns the absolute path to the user's real profile
	// for this browser, or ErrNoSeedProfile if none can be found.
	seedProfile() (string, error)
	// launchArgs builds the exec command and argv to open urls in a
	// window backed by profileDir.
	launchArgs(profileDir string, urls []string, newWindow bool) (cmd string, args []string)
	// cacheExcludes lists profile-relative directories that hold only
	// regenerable cache/crash data.
	cacheExcludes() []string
}

// firefoxFamily and chromiumFamily map a binary basename to its family.
// Kept here (rather than reusing registry's list) so the browser
// package has no dependency on registry.
var firefoxFamily = map[string]string{
	"firefox":     "firefox",
	"firefox-esr": "firefox",
	"zen":         "zen",
	"zen-browser": "zen",
	"librewolf":   "librewolf",
	"waterfox":    "waterfox",
	"floorp":      "floorp",
	"icecat":      "firefox",
}

var chromiumFamily = map[string]string{
	"chromium":             "chromium",
	"chromium-browser":     "chromium",
	"google-chrome":        "google-chrome",
	"google-chrome-stable": "google-chrome",
	"brave":                "brave",
	"brave-browser":        "brave",
	"vivaldi":              "vivaldi",
	"vivaldi-stable":       "vivaldi",
	"microsoft-edge":       "microsoft-edge",
}

// adapterFor selects the adapter for a browser binary. home is the
// user's home directory (injected so tests can point discovery at a
// temp tree without touching the real one).
func adapterFor(bin, home string) (adapter, error) {
	base := filepath.Base(bin)
	if key, ok := firefoxFamily[base]; ok {
		return &geckoAdapter{bin: bin, key: key, home: home}, nil
	}
	if _, ok := chromiumFamily[base]; ok {
		return &chromiumAdapter{bin: bin, base: base, home: home}, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedBrowser, base)
}

// --- Firefox / Gecko family -------------------------------------------------

type geckoAdapter struct {
	bin  string
	key  string // "zen", "firefox", "librewolf", ...
	home string
}

// geckoRoot returns the base directory holding profiles.ini for this
// Gecko browser (e.g. ~/.zen, ~/.mozilla/firefox, ~/.librewolf).
func (g *geckoAdapter) geckoRoot() string {
	base := filepath.Join(g.home, ".mozilla", "firefox")
	if runtime.GOOS == "darwin" {
		base = filepath.Join(g.home, "Library", "Application Support", "Firefox")
	} else if runtime.GOOS == "windows" {
		cfg := os.Getenv("APPDATA")
		if cfg == "" {
			cfg = filepath.Join(g.home, "AppData", "Roaming")
		}
		base = filepath.Join(cfg, "Mozilla", "Firefox")
	}
	switch g.key {
	case "zen":
		if runtime.GOOS == "darwin" {
			return filepath.Join(g.home, "Library", "Application Support", "zen")
		}
		if runtime.GOOS == "windows" {
			cfg := os.Getenv("APPDATA")
			if cfg == "" {
				cfg = filepath.Join(g.home, "AppData", "Roaming")
			}
			return filepath.Join(cfg, "zen")
		}
		return filepath.Join(g.home, ".zen")
	case "librewolf":
		if runtime.GOOS == "darwin" {
			return filepath.Join(g.home, "Library", "Application Support", "LibreWolf")
		}
		if runtime.GOOS == "windows" {
			cfg := os.Getenv("APPDATA")
			if cfg == "" {
				cfg = filepath.Join(g.home, "AppData", "Roaming")
			}
			return filepath.Join(cfg, "librewolf")
		}
		return filepath.Join(g.home, ".librewolf")
	case "waterfox":
		if runtime.GOOS == "darwin" {
			return filepath.Join(g.home, "Library", "Application Support", "Waterfox")
		}
		if runtime.GOOS == "windows" {
			cfg := os.Getenv("APPDATA")
			if cfg == "" {
				cfg = filepath.Join(g.home, "AppData", "Roaming")
			}
			return filepath.Join(cfg, "Waterfox")
		}
		return filepath.Join(g.home, ".waterfox")
	case "floorp":
		if runtime.GOOS == "darwin" {
			return filepath.Join(g.home, "Library", "Application Support", "Floorp")
		}
		if runtime.GOOS == "windows" {
			cfg := os.Getenv("APPDATA")
			if cfg == "" {
				cfg = filepath.Join(g.home, "AppData", "Roaming")
			}
			return filepath.Join(cfg, "Floorp")
		}
		return filepath.Join(g.home, ".floorp")
	default: // firefox and icecat use the mozilla dir
		return base
	}
}

func (g *geckoAdapter) seedProfile() (string, error) {
	root := g.geckoRoot()
	ini := filepath.Join(root, "profiles.ini")
	rel, err := defaultProfileFromINI(ini)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(root, rel)
	if fi, statErr := os.Stat(abs); statErr != nil || !fi.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrNoSeedProfile, abs)
	}
	return abs, nil
}

func (g *geckoAdapter) launchArgs(profileDir string, urls []string, newWindow bool) (string, []string) {
	// A unique --profile forces a brand-new, dia-owned instance rather
	// than remoting into the user's running browser, so there is
	// exactly one window and the tab-dispatch race the legacy path
	// worked around cannot happen: pass every URL in one invocation.
	args := []string{"--profile", profileDir, "--no-remote", "--new-instance"}
	for i, u := range urls {
		flag := "--new-tab"
		if i == 0 {
			flag = "--new-window"
		}
		args = append(args, flag, u)
	}
	return g.bin, args
}

func (g *geckoAdapter) cacheExcludes() []string {
	return []string{
		"cache2", "startupCache", "shader-cache", "OfflineCache",
		"minidumps", "Crash Reports", "datareporting",
		"saved-telemetry-pings", "Pending Pings",
	}
}

// --- Chromium family --------------------------------------------------------

type chromiumAdapter struct {
	bin  string
	base string
	home string
}

// chromiumConfigDir returns the user-data-dir for this Chromium browser.
func (c *chromiumAdapter) chromiumConfigDir() string {
	cfg := filepath.Join(c.home, ".config")
	if runtime.GOOS == "darwin" {
		cfg = filepath.Join(c.home, "Library", "Application Support")
	} else if runtime.GOOS == "windows" {
		cfg = os.Getenv("LOCALAPPDATA")
		if cfg == "" {
			cfg = filepath.Join(c.home, "AppData", "Local")
		}
	}
	switch chromiumFamily[c.base] {
	case "google-chrome":
		if runtime.GOOS == "windows" {
			return filepath.Join(cfg, "Google", "Chrome", "User Data")
		}
		return filepath.Join(cfg, "google-chrome")
	case "brave":
		if runtime.GOOS == "windows" {
			return filepath.Join(cfg, "BraveSoftware", "Brave-Browser", "User Data")
		}
		return filepath.Join(cfg, "BraveSoftware", "Brave-Browser")
	case "vivaldi":
		if runtime.GOOS == "windows" {
			return filepath.Join(cfg, "Vivaldi", "User Data")
		}
		return filepath.Join(cfg, "vivaldi")
	case "microsoft-edge":
		if runtime.GOOS == "windows" {
			return filepath.Join(cfg, "Microsoft", "Edge", "User Data")
		}
		return filepath.Join(cfg, "microsoft-edge")
	default:
		return filepath.Join(cfg, "chromium")
	}
}

func (c *chromiumAdapter) seedProfile() (string, error) {
	dir := c.chromiumConfigDir()
	// "Local State" is present in every real Chromium user-data-dir and
	// is the least ambiguous marker that this is one.
	if fi, err := os.Stat(filepath.Join(dir, "Local State")); err != nil || fi.IsDir() {
		if fi2, err2 := os.Stat(filepath.Join(dir, "Default")); err2 != nil || !fi2.IsDir() {
			return "", fmt.Errorf("%w: %s", ErrNoSeedProfile, dir)
		}
	}
	return dir, nil
}

func (c *chromiumAdapter) launchArgs(profileDir string, urls []string, newWindow bool) (string, []string) {
	// A fresh --user-data-dir forces a new owned process. Multiple
	// positional URLs open as tabs in one window; --new-window makes
	// the window explicit.
	args := []string{"--user-data-dir=" + profileDir, "--new-window"}
	args = append(args, urls...)
	return c.bin, args
}

func (c *chromiumAdapter) cacheExcludes() []string {
	return []string{
		"Default/Cache", "Default/Code Cache", "Default/GPUCache",
		"Default/Service Worker/CacheStorage", "ShaderCache",
		"GrShaderCache", "GraphiteDawnCache", "component_crx_cache",
		"Crashpad", "Safe Browsing",
	}
}

// defaultProfileFromINI parses a Firefox-family profiles.ini and returns
// the relative Path of the default profile. It honors the install's
// locked default first (the [InstallXXXX] Default= key, which is what a
// running browser actually uses), then falls back to the [ProfileN]
// entry marked Default=1, then the first profile listed.
func defaultProfileFromINI(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoSeedProfile, err)
	}
	defer f.Close()

	type profile struct {
		relPath   string
		isDefault bool
	}
	var profiles []profile
	var installDefault string
	var firstPath string

	section := ""
	cur := profile{}
	flush := func() {
		if strings.HasPrefix(section, "Profile") && cur.relPath != "" {
			profiles = append(profiles, cur)
			if firstPath == "" {
				firstPath = cur.relPath
			}
		}
		cur = profile{}
	}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section = strings.Trim(line, "[]")
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		switch {
		case strings.HasPrefix(section, "Profile"):
			switch key {
			case "Path":
				cur.relPath = val
			case "Default":
				cur.isDefault = val == "1"
			}
		case strings.HasPrefix(section, "Install"):
			if key == "Default" {
				installDefault = val
			}
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return "", err
	}

	if installDefault != "" {
		return installDefault, nil
	}
	for _, p := range profiles {
		if p.isDefault {
			return p.relPath, nil
		}
	}
	if firstPath != "" {
		return firstPath, nil
	}
	return "", fmt.Errorf("%w: no profiles in %s", ErrNoSeedProfile, path)
}
