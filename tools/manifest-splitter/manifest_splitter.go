package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	kindLinePrefix = "kind: "

	yamlSeparator = "---"
)

var (
	operatorName  string
	manifestsFile string
	outputDir     string
	csvExtension  string
)

func main() {
	var manifests io.Reader = os.Stdin
	if manifestsFile != "" {
		fin, err := os.Open(manifestsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open %s; %v", manifestsFile, err)
			os.Exit(1)
		}

		defer fin.Close()

		manifests = fin
	}

	err := splitFile(manifests, operatorName, outputDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	flag.StringVar(&operatorName, "operator-name", "", "The name of the operator to use; mandatory")
	flag.StringVar(&manifestsFile, "manifests-file", "", "The path to the manifests file; optional; if not provided, reads from stdin")
	flag.StringVar(&outputDir, "output-dir", ".", "The directory to write the split manifests to; default: the working directory")
	flag.StringVar(&csvExtension, "csv-extension", ".clusterserviceversion.yaml", "The file extension to use for CSV files; default: '.clusterserviceversion.yaml'")
	flag.Parse()

	if operatorName == "" {
		fmt.Fprintln(os.Stderr, "Please provide the operator name using the operator-name flag")
		flag.Usage()
		os.Exit(1)
	}
}

func splitFile(fin io.Reader, operatorName, outputDir string) error {
	scanner := bufio.NewScanner(fin)
	fh := newFileHandler(operatorName, outputDir)

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("unexpected error while processing input; %v", err)
		}

		line := scanner.Text()

		if line == yamlSeparator {
			if !fh.isEmpty() {
				if err := tryWriteFile(fh); err != nil {
					return err
				}

				fh = newFileHandler(operatorName, outputDir)
			}
			continue
		}

		fh.writeLine(line)

		if strings.HasPrefix(line, kindLinePrefix) {
			if fh.fileTypeIsSet {
				return fmt.Errorf(`wrong manifest: multiple "kind" lines of %q`, line)
			}
			fh.setFileType(strings.ToLower(strings.TrimPrefix(line, kindLinePrefix)))
		}
	}

	if !fh.isEmpty() {
		if err := tryWriteFile(fh); err != nil {
			return err
		}
	}

	return nil
}

func tryWriteFile(fh *fileHandler) error {
	if err := fh.writeToFile(); err != nil {
		return fmt.Errorf("failed to write to file %s; %v", fh.getFileName(), err)
	}

	if !fh.fileTypeIsSet {
		fmt.Fprintln(os.Stderr, "Found a non-kubernetes file. It will be saved as", fh.getFileName())
	}

	return nil
}
