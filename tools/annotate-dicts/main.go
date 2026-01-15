package main

import (
	"context"
	"log"
	"os"

	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/annotate"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/config"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/imagestream"
)

func main() {
	config.InitFlags()

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
			annotate.PrintErrorAndExit("error building image stream map: %v", err)
		}

		log.Println("Successfully read the imageStream files")
	}

	entries, err := os.ReadDir(config.DictDir())
	if err != nil {
		annotate.PrintErrorAndExit("error reading the DataImportCronTemplate directory %s: %v", config.DictDir(), err)
	}

	var outputFile *os.File
	if config.OutputFileName() != "" {
		outputFile, err = os.Create(config.OutputFileName())
		if err != nil {
			annotate.PrintErrorAndExit("error creating file %s: %v", config.OutputFileName(), err)
		}
		defer outputFile.Close()
	}

	for _, entry := range entries {
		if err = annotate.AnnotateOneFile(ctx, isMap, entry, outputFile); err != nil {
			annotate.PrintErrorAndExit("error annotating file %s: %v", entry.Name(), err)
		}
	}
}
