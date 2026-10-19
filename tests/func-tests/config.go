package tests

import (
	"flag"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	ConfigFileFlag = "config-file"
)

var (
	configFileName string
	config         *TestConfig
)

type QuickStartTestItem struct {
	Name        string `yaml:"name,omitempty"`
	DisplayName string `yaml:"displayName,omitempty"`
}

type QuickStartTestConfig struct {
	TestItems []QuickStartTestItem `yaml:"testItems,omitempty"`
}

type DashboardTestItem struct {
	Name      string `yaml:"name,omitempty"`
	Namespace string `yaml:"namespace,omitempty"`
	// keys expected in the configmap
	Keys []string `yaml:"keys,omitempty"`
}

type DashboardTestConfig struct {
	TestItems []DashboardTestItem `yaml:"testItems,omitempty"`
}

type DataImportCronConfig struct {
	ExpectedDataImportCrons []string            `yaml:"expectedDataImportCrons,omitempty"`
	ExpectedImageStream     []ImageStreamConfig `yaml:"expectedImageStream,omitempty"`
	Namespace               string              `yaml:"namespace,omitempty"`
}

type ImageStreamConfig struct {
	Name         string   `yaml:"name"`
	RegistryName string   `yaml:"registryName"`
	UsageImages  []string `yaml:"usageImages,omitempty"`
}

type GuestDeviceDriversTestConfig struct {
	// ExpectedConfigMap is the name of the guest device drivers reference ConfigMap the
	// operator is expected to deploy. Leave it empty when the operator image does not ship
	// the build-time injected drivers.yaml reference file, and the ConfigMap is not
	// expected to exist.
	ExpectedConfigMap string `yaml:"expectedConfigMap,omitempty"`
}

type TestConfig struct {
	QuickStart         QuickStartTestConfig         `yaml:"quickStart,omitempty"`
	Dashboard          DashboardTestConfig          `yaml:"dashboard,omitempty"`
	DataImportCron     DataImportCronConfig         `yaml:"dataImportCron,omitempty"`
	GuestDeviceDrivers GuestDeviceDriversTestConfig `yaml:"guestDeviceDrivers,omitempty"`
}

func init() {
	flag.StringVar(&configFileName, ConfigFileFlag, "", "File contains test configuration")
}

func GetConfig() *TestConfig {
	once := sync.Once{}
	once.Do(func() {
		config = loadConfig(configFileName)
	})

	return config
}

func loadConfig(fileName string) *TestConfig {
	cfg := TestConfig{}

	if fileName != "" {
		file, err := os.Open(fileName)
		if err != nil {
			panic(err)
		}
		dec := yaml.NewDecoder(file)
		err = dec.Decode(&cfg)
		if err != nil {
			panic(err)
		}
	}

	return &cfg
}
