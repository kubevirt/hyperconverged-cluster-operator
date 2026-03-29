package annotate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/config"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/dicts"
	"golang.org/x/sync/errgroup"
)

func AnnotateOneFile(ctx context.Context, isMap map[string]string, entry os.DirEntry, outputFile *os.File) error {

	fmt.Printf("===== annotating %s\n", entry.Name())
	fmt.Printf("with map %v\n", isMap)

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

func writeResult(res []byte, filename string, outputFile *os.File) error {
	var (
		out io.Writer
		err error
	)

	if config.ShouldUpdate() {
		out, err = os.Create(filename)
		if err != nil {
			PrintErrorAndExit("error creating file %s: %v", filename, err)
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

func PrintErrorAndExit(template string, args ...any) {
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
