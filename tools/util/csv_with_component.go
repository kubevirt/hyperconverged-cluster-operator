package util

import (
	"flag"
	"fmt"
	"os"

	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

type CsvWithComponent struct {
	Name      string
	Csv       string
	Component hcoutil.AppComponent
}

var (
	cnaCsv      = flag.String("cna-csv", "", "Cluster Network Addons CSV string")
	cnaCsvFile  = flag.String("cna-csv-file", "", "Cluster Network Addons CSV yaml file")
	virtCsv     = flag.String("virt-csv", "", "KubeVirt CSV string")
	virtCsvFile = flag.String("virt-csv-file", "", "KubeVirt CSV yaml file")
	sspCsv      = flag.String("ssp-csv", "", "Scheduling Scale Performance CSV string")
	sspCsvFile  = flag.String("ssp-csv-file", "", "Scheduling Scale Performance CSV yaml file")
	cdiCsv      = flag.String("cdi-csv", "", "Containerized Data Importer CSV String")
	cdiCsvFile  = flag.String("cdi-csv-file", "", "Containerized Data Importer CSV yaml file")
	hppCsv      = flag.String("hpp-csv", "", "HostPath Provisioner Operator CSV String")
	hppCsvFile  = flag.String("hpp-csv-file", "", "HostPath Provisioner Operator CSV yaml file")
	ttoCsv      = flag.String("tto-csv", "", "Tekton tasks operator CSV string")
	ttoCsvFile  = flag.String("tto-csv-file", "", "Tekton tasks Operator CSV yaml file")
)

func GetInitialCsvList() ([]CsvWithComponent, error) {
	err := getAllCSVs()

	if err != nil {
		return nil, err
	}

	return []CsvWithComponent{
		{
			Name:      "CNA",
			Csv:       *cnaCsv,
			Component: hcoutil.AppComponentNetwork,
		},
		{
			Name:      "KubeVirt",
			Csv:       *virtCsv,
			Component: hcoutil.AppComponentCompute,
		},
		{
			Name:      "SSP",
			Csv:       *sspCsv,
			Component: hcoutil.AppComponentSchedule,
		},
		{
			Name:      "TTO",
			Csv:       *ttoCsv,
			Component: hcoutil.AppComponentTekton,
		},
		{
			Name:      "CDI",
			Csv:       *cdiCsv,
			Component: hcoutil.AppComponentStorage,
		},
		{
			Name:      "HPP",
			Csv:       *hppCsv,
			Component: hcoutil.AppComponentStorage,
		},
	}, nil
}

func getAllCSVs() error {
	for _, f := range []struct {
		str      *string
		fileName string
		flagName string
	}{
		{str: cnaCsv, fileName: *cnaCsvFile, flagName: "cna-csv"},
		{str: virtCsv, fileName: *virtCsvFile, flagName: "virt-csv"},
		{str: sspCsv, fileName: *sspCsvFile, flagName: "ssp-csv"},
		{str: cdiCsv, fileName: *cdiCsvFile, flagName: "cdi-csv"},
		{str: hppCsv, fileName: *hppCsvFile, flagName: "hpp-csv"},
		{str: ttoCsv, fileName: *ttoCsvFile, flagName: "tto-csv"},
	} {
		if err := fileOrString(f.str, f.fileName, f.flagName); err != nil {
			return err
		}
	}
	return nil
}

func fileOrString(str *string, fileName, csvName string) error {
	if (*str == "") == (fileName == "") {
		return fmt.Errorf(`one and only one of the "--%[1]s" and the "--%[1]s-file" flags must be used`, csvName)
	}

	if *str != "" {
		return nil
	}

	csvFile, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("can't read %q; %w", fileName, err)
	}

	*str = string(csvFile)

	return nil
}
