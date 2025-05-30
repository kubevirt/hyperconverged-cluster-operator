package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/config"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/dicts"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/imagestream"
)

func main() {
	var (
		isMap map[string]string
		err   error
	)

	log.Println("Start annotating the DataImportCronTemplate files")
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout())
	defer cancel()

	if config.ImageStreamDir() != "" {
		log.Println("Reading the imageStream files...")

		isMap, err = imagestream.BuildImageStreamMap()
		if err != nil {
			printErrorAndExit("error building image stream map: %v", err)
		}

		log.Println("Successfully read the imageStream files")
	}

	entries, err := os.ReadDir(config.DictDir())
	if err != nil {
		printErrorAndExit("error reading the DataImportCronTemplate directory %s: %v", config.DictDir(), err)
	}

	var outputFile *os.File
	if config.OutputFileName() != "" {
		outputFile, err = os.Create(config.OutputFileName())
		if err != nil {
			printErrorAndExit("error creating file %s: %v", config.OutputFileName(), err)
		}
		defer outputFile.Close()
	}

	for _, entry := range entries {
		if err = annotateOneFile(ctx, isMap, entry, outputFile); err != nil {
			printErrorAndExit(err.Error())
		}
	}
}

func annotateOneFile(ctx context.Context, isMap map[string]string, entry os.DirEntry, outputFile *os.File) error {
	if entry.IsDir() {
		return nil
	}

	filename := path.Join(config.DictDir(), entry.Name())
	if ext := path.Ext(filename); ext != ".yaml" && ext != ".yml" {
		return nil
	}

	group, dictsCtx := errgroup.WithContext(ctx)
	ds, err := dicts.NewDicts(group, filename)
	if err != nil {
		return fmt.Errorf("error parsing the DataImportCronTemplate file %s: %v", filename, err)
	}

	changed, err := ds.Run(dictsCtx, isMap)
	if err != nil {
		return errors.New(err.Error())
	}

	if !changed {
		log.Printf("no changes for %s", filename)
		return nil
	}

	res, err := ds.ToYaml()
	if err != nil {
		return fmt.Errorf("error converting DataImportCronTemplate to yaml: %v", err)
	}

	err = writeResult(res, filename, outputFile)
	if err != nil {
		return errors.New(err.Error())
	}

	return nil
}

func printErrorAndExit(template string, args ...any) {
	if !strings.HasSuffix(template, "\n") {
		template += "\n"
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, template)
	} else {
		fmt.Fprintf(os.Stderr, template, args...)
	}
	os.Exit(1)
}

func writeResult(res []byte, filename string, outputFile *os.File) error {
	var (
		out io.Writer
		err error
	)

	if config.ShouldUpdate() {
		out, err = os.Create(filename)
		if err != nil {
			printErrorAndExit("error creating file %s: %v", filename, err)
		}
		log.Printf("Updating the DataImportCronTemplate file %s with the changes", filename)
		defer out.(*os.File).Close()
	} else if outputFile != nil {
		log.Printf("Writing the updated DataImportCronTemplates to %s", outputFile.Name())
		out = outputFile
	} else {
		out = os.Stdout
	}

	_, err = out.Write(res)
	if err != nil {
		return fmt.Errorf("error writing the result %s: %v", filename, err)
	}

	return nil
}
