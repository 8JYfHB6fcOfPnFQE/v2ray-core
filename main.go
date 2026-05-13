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
	// Default confdir to ~/.v2ray for personal convenience
	defaultConfDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		defaultConfDir = filepath.Join(home, ".v2ray")
		if _, err := os.Stat(defaultConfDir); os.IsNotExist(err) {
			defaultConfDir = ""
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

	// Graceful shutdown on signal
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals
}

// printVersion outputs the current version and build info.
func printVersion() {
	version := core.VersionStatement()
	for _, s := range version {
		fmt.Println(s)
	}
}

// startV2Ray initializes and returns a V2Ray server instance.
func startV2Ray() (core.Server, error) {
	opts, err := parseFlags()
	if err != nil {
		return nil, err
	}

	config, err := core.LoadConfig(opts.format, opts.configFiles[0], opts.configFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to read config files: [%s]. Cause: %s", strings.Join(opts.configFiles, ", "), err)
	}

	server, err := core.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %s", err)
	}

	return server, nil
}

type options struct {
	configFiles []string
	format      string
}

// parseFlags resolves config file paths from flags and config directory.
func parseFlags() (*options, error) {
	opts := &options{
		format: *format,
	}

	if configDir != "" {
		dirEntries, err := os.ReadDir(configDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read config directory: %s", err)
		}
		for _, entry := range dirEntries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
				opts.configFiles = append
