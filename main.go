package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	syscall "syscall"

	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/all"
)

var (
	configFiles cmdarg // -config
	configDir   string // -confdir
	version     = flag.Bool("version", false, "Show current version of V2Ray.")
	testConfig  = flag.Bool("test", false, "Test config file only, without launching V2Ray server.")
	format      = flag.String("format", "json", "Format of input file.")
)

// cmdarg holds multiple config file arguments
type cmdarg []string

func (c *cmdarg) String() string {
	return strings.Join(*c, ",")
}

func (c *cmdarg) Set(value string) error {
	*c = append(*c, value)
	return nil
}

func init() {
	flag.Var(&configFiles, "config", "Config file for V2Ray. Multiple assign is accepted (only json). Latter ones overrides the former ones.")
	flag.Var(&configFiles, "c", "Short alias of -config")
	// Default confdir to ~/.v2ray for personal convenience.
	// Also check XDG_CONFIG_HOME/v2ray as an alternative location.
	defaultConfDir := ""
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		xdgV2ray := filepath.Join(xdgConfig, "v2ray")
		if _, err := os.Stat(xdgV2ray); err == nil {
			defaultConfDir = xdgV2ray
		}
	}
	if defaultConfDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultConfDir = filepath.Join(home, ".v2ray")
			if _, err := os.Stat(defaultConfDir); os.IsNotExist(err) {
				defaultConfDir = ""
			}
		}
	}
	flag.StringVar(&configDir, "confdir", defaultConfDir, "A directory with multiple json config")
}

func main() {
	flag.Parse()

	printVersion()

	if *version {
		return
	}

	server, err := startV2Ray()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		// Configuration error. Exit with a special value to prevent systemd from restarting.
		os.Exit(23)
	}

	if *testConfig {
		fmt.Println("Configuration OK.")
		return
	}

	if err := server.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to start V2Ray:", err)
		os.Exit(1)
	}
	defer server.Close()

	// Graceful shutdown on signal.
	// Also handle SIGHUP so the process can be cleanly stopped by service managers
	// that send SIGHUP before SIGTERM (e.g. some init systems).
	// Note: on my machine I also added SIGUSR1 here previously for manual reload
	// testing, but keeping it simple with just the standard signals for now.
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	<-osSignals
	fmt.Fprintln(os.Stderr, "Shutting down V2Ray...")

	// Print runtime info on shutdown for easier debugging on my machines.
	fmt.Fprintf(os.Stderr, "Runtime: %s/%s, goroutines at shutdown: %d\n",
		runtime.GOOS, runtime.GOARCH, runtime.NumGoroutine())
}

// printVersion outputs the current version and build info.
func printVersion() {
	v := core.VersionStatement()
	for _, s := range v {
		fmt.Println(s)
	}
}

// startV2Ray loads configuration and initializes the V2Ray server instance.
func startV2Ray() (core.Server, error) {
	configFiles := getConfigFilePath()

	config, err := serial.LoadJSONConfig(configFiles)
	if err != nil {
		return nil, err
	}

	return core.New(config)
}
