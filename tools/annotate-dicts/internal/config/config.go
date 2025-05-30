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
	cfg config
)

type config struct {
	imageStreamDir string
	dictDir        string
	shouldUpdate   bool
	outputFileName string
	timeout        time.Duration
}

func ImageStreamDir() string {
	return cfg.imageStreamDir
}

func DictDir() string {
	return cfg.dictDir
}

func ShouldUpdate() bool {
	return cfg.shouldUpdate
}

func OutputFileName() string {
	return cfg.outputFileName
}

func Timeout() time.Duration {
	return cfg.timeout
}

func init() {
	flag.StringVar(&cfg.imageStreamDir, "image-stream-dir", "", "Directory containing image stream files")
	flag.StringVar(&cfg.dictDir, "dict-dir", "", "Directory containing DataImportCronTemplate files (required)")
	flag.StringVar(&cfg.outputFileName, "output", "", "path to output file. Can't be used with the -i flag")
	flag.DurationVar(&cfg.timeout, "timeout", 5*time.Minute, "Timeout for the operation")
	flag.BoolFunc("i", "Update the DataImportCronTemplate files with the updated architectures. Can't be used with the --output flag", parseBoolFlag(&cfg.shouldUpdate))

	flag.Parse()

	if cfg.dictDir == "" {
		printUsageAndErrorAndExit("the --dict-dir parameter is required")
	}

	if cfg.shouldUpdate && cfg.outputFileName != "" {
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
