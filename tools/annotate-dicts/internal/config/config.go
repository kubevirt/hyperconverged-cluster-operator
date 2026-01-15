package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	cfg Config
)

type Config struct {
	ImageStreamDir string
	DictDir        string
	ShouldUpdate   bool
	OutputFileName string
	Timeout        time.Duration
	ImportsToKeep  int
}

func GetConfig() *Config {
	return &cfg
}

func ImageStreamDir() string {
	return cfg.ImageStreamDir
}

func DictDir() string {
	return cfg.DictDir
}

func ShouldUpdate() bool {
	return cfg.ShouldUpdate
}

func OutputFileName() string {
	return cfg.OutputFileName
}

func Timeout() time.Duration {
	return cfg.Timeout
}

func ImportsToKeep() int {
	return cfg.ImportsToKeep
}

func InitFlags() {
	flag.StringVar(&cfg.ImageStreamDir, "image-stream-dir", "", "Directory containing image stream files")
	flag.StringVar(&cfg.DictDir, "dict-dir", "", "Directory containing DataImportCronTemplate files (required)")
	flag.StringVar(&cfg.OutputFileName, "output", "", "path to output file. Can't be used with the -i flag")
	flag.DurationVar(&cfg.Timeout, "timeout", 5*time.Minute, "Timeout for the operation")
	flag.BoolFunc("i", "Update the DataImportCronTemplate files with the updated architectures. Can't be used with the --output flag", parseBoolFlag(&cfg.ShouldUpdate))
	flag.IntVar(&cfg.ImportsToKeep, "imports-to-keep", 1, "Value to set for spec.importsToKeep in all DataImportCronTemplate objects")

	flag.Parse()

	if cfg.DictDir == "" {
		printUsageAndErrorAndExit("the --dict-dir parameter is required")
	}

	if cfg.ShouldUpdate && cfg.OutputFileName != "" {
		printUsageAndErrorAndExit("can't use the --output parameter with the -i parameter")
	}
}

func printUsageAndErrorAndExit(template string, args ...any) {
	if !strings.HasSuffix(template, "\n") {
		template += "\n"
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, template)
	} else {
		fmt.Fprintf(os.Stderr, template, args...)
	}

	flag.Usage()
	os.Exit(1)
}

func parseBoolFlag(b *bool) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			*b = true
			return nil
		}
		parseBool, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("error parsing %q as boolean", s)
		}
		*b = parseBool
		return nil
	}
}
