//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// winProcess is the ProcessHandle for the Windows implementation.
type winProcess struct {
	cmd  *exec.Cmd
	pid  int
	job  windows.Handle
	done chan struct{}
}

func (p *winProcess) PID() int              { return p.pid }
func (p *winProcess) Done() <-chan struct{} { return p.done }

type winPlatform struct {
	cmdRunner
	mu   sync.Mutex
	jobs map[int]windows.Handle
}

func newWinPlatform() Platform { return &winPlatform{jobs: make(map[int]windows.Handle)} }

func (p *winPlatform) Launch(opts LaunchOpts) (ProcessHandle, error) {
	if opts.Cmd == "" {
		return nil, fmt.Errorf("launch: empty command")
	}
	cmd := exec.Command(opts.Cmd, opts.Args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	// CREATE_NEW_PROCESS_GROUP so Ctrl+C in dia does not
	// propagate to the child, and so taskkill /T reaches the
	// tree reliably.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch %s: %w", opts.Cmd, err)
	}
	pid := cmd.Process.Pid
	job, err := createProcessJob(uint32(pid))
	if err != nil {
		// The process started before the job could be configured. Tear down
		// the complete tree so a partial launch cannot leak descendants.
		_ = terminateTree(pid)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("own process tree %d: %w", pid, err)
	}
	p.mu.Lock()
	if p.jobs == nil {
		p.jobs = make(map[int]windows.Handle)
	}
	p.jobs[pid] = job
	p.mu.Unlock()
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		if current, ok := p.jobs[pid]; ok && current == job {
			delete(p.jobs, pid)
			_ = windows.CloseHandle(job)
		}
		p.mu.Unlock()
		close(done)
	}()
	return &winProcess{cmd: cmd, pid: pid, job: job, done: done}, nil
}

func (*winPlatform) IsRunning(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	// tasklist filters by PID and prints CSV. /NH suppresses the
	// header. We look for the PID in the output to decide.
	out, err := exec.Command(
		"tasklist",
		"/FI", fmt.Sprintf("PID eq %d", pid),
		"/NH", "/FO", "CSV",
	).Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), strconv.Itoa(pid)), nil
}

func (p *winPlatform) Kill(pid int, force bool) error {
	if pid <= 0 {
		return nil
	}
	// A live process launched by this daemon is owned by a Job Object. Keep
	// the map lock while terminating so the Wait goroutine cannot close the
	// handle underneath us. Recovered processes have no handle and use the
	// identity-checked taskkill path below.
	p.mu.Lock()
	job := p.jobs[pid]
	if force && job != 0 {
		err := windows.TerminateJobObject(job, 1)
		p.mu.Unlock()
		if err != nil {
			return fmt.Errorf("terminate job for %d: %w", pid, err)
		}
		return nil
	}
	p.mu.Unlock()
	// /T kills the process tree; /F forces. We always force on
	// the second call after a graceful stop or for a recovered process.
	if err := terminateTreeWithForce(pid, force); err != nil {
		return err
	}
	return nil
}

func terminateTree(pid int) error {
	return terminateTreeWithForce(pid, true)
}

func terminateTreeWithForce(pid int, force bool) error {
	args := []string{"/T", "/PID", strconv.Itoa(pid)}
	if force {
		args = append([]string{"/F"}, args...)
	}
	cmd := exec.Command("taskkill", args...)
	if err := cmd.Run(); err != nil {
		// Exit code 128 + signal usually means the process is
		// already gone; treat that as success.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return nil
		}
		return fmt.Errorf("taskkill %d: %w", pid, err)
	}
	return nil
}

func createProcessJob(pid uint32) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("configure job: %w", err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	defer func() { _ = windows.CloseHandle(process) }()
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func (*winPlatform) OpenURL(url string) error {
	// The leading empty "" is the window title; required so cmd
	// does not try to interpret the URL as the title.
	return runDetached("cmd", "/c", "start", "", url)
}

func (*winPlatform) RevealInFileManager(path string) error {
	return runDetached("explorer", path)
}

func (*winPlatform) OpenFile(path string) error {
	return runDetached("cmd", "/c", "start", "", path)
}

func runDetached(prog string, args ...string) error {
	cmd := exec.Command(prog, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", prog, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
