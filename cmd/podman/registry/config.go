package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/rootless"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
)

const (
	ParentNSRequired  = "ParentNSRequired"
	UnshareNSRequired = "UnshareNSRequired"
)

var (
	podmanOptions entities.PodmanConfig
	podmanSync    sync.Once
	abiSupport    = false
)

// PodmanConfig returns an entities.PodmanConfig built up from
// environment and CLI
func PodmanConfig() *entities.PodmanConfig {
	podmanSync.Do(newPodmanConfig)
	return &podmanOptions
}

func newPodmanConfig() {
	if err := setXdgDirs(); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	var mode entities.EngineMode
	switch runtime.GOOS {
	case "darwin", "windows":
		mode = entities.TunnelMode
	case "linux":
		// Some linux clients might only be compiled without ABI
		// support (e.g., podman-remote).
		if abiSupport && !IsRemote() {
			mode = entities.ABIMode
		} else {
			mode = entities.TunnelMode
		}

	default:
		fmt.Fprintf(os.Stderr, "%s is not a supported OS", runtime.GOOS)
		os.Exit(1)
	}

	cfg, err := config.NewConfig("")
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed to obtain podman configuration: "+err.Error())
		os.Exit(1)
	}

	cfg.Network.NetworkConfigDir = cfg.Network.CNIPluginDirs[0]
	if rootless.IsRootless() {
		cfg.Network.NetworkConfigDir = ""
	}

	podmanOptions = entities.PodmanConfig{Config: cfg, EngineMode: mode}
}

// setXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the containers.conf configuration file.
func setXdgDirs() error {
	if !rootless.IsRootless() {
		return nil
	}

	// Setup XDG_RUNTIME_DIR
	if _, found := os.LookupEnv("XDG_RUNTIME_DIR"); !found {
		dir, err := util.GetRuntimeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_RUNTIME_DIR", dir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_RUNTIME_DIR="+dir)
		}
	}

	if _, found := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !found {
		sessionAddr := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "bus")
		if _, err := os.Stat(sessionAddr); err == nil {
			os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+sessionAddr)
		}
	}

	// Setup XDG_CONFIG_HOME
	if _, found := os.LookupEnv("XDG_CONFIG_HOME"); !found {
		cfgHomeDir, err := util.GetRootlessConfigHomeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return errors.Wrapf(err, "cannot set XDG_CONFIG_HOME="+cfgHomeDir)
		}
	}
	return nil
}
