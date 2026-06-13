//go:build windows

package daemon

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "ShadowDaemon"

// InstallService installs Shadow as a Windows service.
// The service is configured to start automatically at boot and runs
// "shadow serve" as its entry point.
func InstallService(execPath string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Service Control Manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.CreateService(serviceName, execPath, mgr.Config{
		DisplayName: "Shadow Daemon",
		Description: "AI Agent Memory Layer — captures corrections and creates persistent rules across coding tools",
		StartType:   mgr.StartAutomatic,
	}, "serve")
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Set recovery actions: restart on failure.
	s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
		{Type: mgr.NoAction, Delay: 0},
	}, 86400) // reset failure count after 24h

	return nil
}

// UninstallService removes the Shadow Windows service.
func UninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Service Control Manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("open service: %w (is it installed? run 'shadow start' first)", err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// StartService starts the Shadow Windows service.
func StartService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("open service: %w (run 'shadow start' first to install)", err)
	}
	defer s.Close()

	return s.Start("serve")
}

// StopService stops the Shadow Windows service.
func StopService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("open service: %w (it may not be installed)", err)
	}
	defer s.Close()

	// Query current status first.
	status, err := s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	// Wait for the service to actually stop (up to 10s).
	timeout := time.Now().Add(10 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(timeout) {
			return fmt.Errorf("timed out waiting for service to stop")
		}
		time.Sleep(250 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("query service status: %w", err)
		}
	}

	return nil
}

// IsWindowsService detects whether the current process is running as a
// Windows service (started by the SCM). When true, the daemon should
// integrate with the service control handler instead of handling signals
// directly.
func IsWindowsService() bool {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return isSvc
}

// RunAsService wraps the daemon's Run method for execution under the
// Windows Service Control Manager.
func RunAsService(ctx context.Context, d *Daemon) error {
	return svc.Run(serviceName, &windowsServiceHandler{ctx: ctx, daemon: d})
}

// windowsServiceHandler adapts a Daemon to the svc.Handler interface.
type windowsServiceHandler struct {
	ctx    context.Context
	daemon *Daemon
}

func (h *windowsServiceHandler) Execute(args []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}

	// Start the daemon in a goroutine; Run blocks until shutdown.
	daemonErr := make(chan error, 1)
	go func() {
		daemonErr <- h.daemon.Run(h.ctx)
	}()

	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				h.daemon.Stop()
				<-daemonErr // Wait for daemon to finish.
				return false, 0
			default:
				status <- c.CurrentStatus
			}
		case err := <-daemonErr:
			if err != nil {
				return false, 1
			}
			return false, 0
		}
	}
}
