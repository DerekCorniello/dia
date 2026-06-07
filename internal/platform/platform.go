// Package platform abstracts OS-specific process launching, signal
// delivery, and shell-outs to open URLs and reveal files. The Runtime
// depends on this interface; concrete implementations live in
// platform_unix.go, platform_linux.go, platform_darwin.go, and
// platform_windows.go.
package platform

// ProcessHandle is a lightweight reference to a launched process.
// PID is what gets stored in the state; Done (if non-nil) closes when
// the process exits. Not all implementations populate Done.
type ProcessHandle interface {
	PID() int
	Done() <-chan struct{}
}

// LaunchOpts describes a single process to start.
type LaunchOpts struct {
	// Cmd is the executable name or path. On Unix it is resolved
	// via $PATH; on Windows the usual PATHEXT rules apply.
	Cmd string

	// Args are passed as argv. Element 0 is NOT the program name
	// (matching exec.Command conventions).
	Args []string

	// Cwd is the working directory. Empty means inherit.
	Cwd string

	// Env is appended to the inherited environment. Format is
	// "KEY=VALUE".
	Env []string
}

// Platform is the OS abstraction the Runtime depends on.
type Platform interface {
	// Launch starts a new detached process. Implementations must
	// arrange for the process to outlive dia (own session on
	// Unix; new process group on Windows) so dia stop can clean
	// up the whole tree.
	Launch(opts LaunchOpts) (ProcessHandle, error)

	// OpenURL asks the OS to open url in the default handler.
	OpenURL(url string) error

	// IsRunning reports whether pid is still alive. Errors other
	// than "not found" are propagated; the caller decides.
	IsRunning(pid int) (bool, error)

	// Kill terminates pid. With force=false, implementations
	// should send SIGTERM (or the Windows equivalent) and give
	// the process a short grace period before escalating. With
	// force=true, SIGKILL/taskkill /F is sent immediately.
	// The entire process tree spawned under dia should be
	// terminated.
	Kill(pid int, force bool) error

	// RevealInFileManager shows path in the OS file manager. If
	// path is a file, the file should be selected; if it is a
	// directory, the directory should be opened.
	RevealInFileManager(path string) error

	// OpenFile opens path with the OS default application for
	// that file type (e.g. a text editor for .json files).
	OpenFile(path string) error
}
