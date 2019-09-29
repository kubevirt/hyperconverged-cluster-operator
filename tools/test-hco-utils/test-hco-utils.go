package main

import (
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	//"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
)

var (
	kubectlCmd                    string
	clusterServiceVersionFileName string
)

// yaml parsing structs (for parsing of cluster service version yaml file)
type hcoType struct {
	Spec hcoSpec `yaml:"spec"`
}

type hcoSpec struct {
	Install hcoInstall `yaml:"install"`
}

type hcoInstall struct {
	Spec hcoInstallSpec `yaml:"spec"`
}

type hcoInstallSpec struct {
	Deployments []hcoDeployments `yaml:"deployments"`
}

type hcoDeployments struct {
	Name string            `yaml:"name"`
	Spec hcoDeploymentSpec `yaml:"spec"`
}

type hcoDeploymentSpec struct {
	Template hcoDeploymentSpecTemplate `yaml:"template"`
}

type hcoDeploymentSpecTemplate struct {
	Spec hcoDeploymentSpecTemplateSpec `yaml:"spec"`
}

type hcoDeploymentSpecTemplateSpec struct {
	Containers []hcoDeploymentContainers `yaml:"containers"`
}

type hcoDeploymentContainers struct {
	Image string `yaml:"image"`
}

type specDeployment struct {
	Name  string
	Image string
}

func parseClusterServiceVersionFile(fname string) ([]specDeployment, error) {
	bdata, err := ioutil.ReadFile(fname)
	if err != nil {
		fmt.Print(err)
		return nil, err

	}
	var data hcoType

	err = yaml.Unmarshal([]byte(bdata), &data)
	if err != nil {
		fmt.Print(err)
		return nil, err
	}

	numResult := len(data.Spec.Install.Spec.Deployments)
        if numResult  == 0 {
                err := fmt.Errorf( "no deployments in spec")
                return nil, err
        }
	retVals := make([]specDeployment, numResult)

	for pos, spec := range data.Spec.Install.Spec.Deployments {
		fmt.Println("****")
		fmt.Println(spec)

		retVals[pos] = specDeployment{spec.Name, spec.Spec.Template.Spec.Containers[0].Image}
		fmt.Println(retVals[pos])
	}

	return retVals, nil
}

// parsing of deployment json
type depRoot struct {
	Items []depItem `json:"items"`
}

type depItem struct {
	Metadata depMetadata `json:"metadata"`
	Spec     depSpec     `json:"spec"`
	Status   depStatus   `json:"status"`
}

type depStatus struct {
	Conditions []depStatusCondition `json:"conditions"`
}

type depStatusCondition struct {
	Status string `json:"status"`
	Type   string `json:"type"`
}

type depMetadata struct {
	Name string `json:"name"`
}

type depSpec struct {
	Template depTemplate `json:"template"`
}

type depTemplate struct {
	Spec depTemplateSpec `json:"spec"`
}

type depTemplateSpec struct {
	Containers []depContainers `json:"containers"`
}

type depContainers struct {
	Image string `json:"Image"`
}

// parsed entity from deployment json
type deploymentData struct {
	Image     string
	Available bool
}

func parseDeployments(bdata string) (map[string]deploymentData, error) {
	var data depRoot

	err := json.Unmarshal([]byte(bdata), &data)
	if err != nil {
		fmt.Print(err)
		return nil, err
	}

	ret := make(map[string]deploymentData)

	for _, entry := range data.Items {
		fmt.Println("###")
		fmt.Println(entry)

		name := entry.Metadata.Name
		imageName := entry.Spec.Template.Spec.Containers[0].Image

		available := false
		for _, cond := range entry.Status.Conditions {
			if cond.Type == "Available" && cond.Status == "True" {
				available = true
			}
		}

		depData := deploymentData{imageName, available}
		fmt.Println(depData)
		ret[name] = depData
	}

	return ret, nil

}

func matchImages(entryImage string, deploymentImage string) bool {
	return entryImage == deploymentImage
}

func matchClusterServiceDataToDeployment(specDep []specDeployment, depData map[string]deploymentData) bool {
	status := true
	for _, entry := range specDep {
		if deploymentEntry, ok := depData[entry.Name]; !ok {
			fmt.Printf("no deployment exists for Cluster service entry %s", entry.Name)
			status = false
		} else {
			if !deploymentEntry.Available {
				fmt.Printf("deployment %s exists, but is is not available", entry.Name)
				status = false
			}
			if !matchImages(entry.Image, deploymentEntry.Image) {
				fmt.Printf("images in cluster service entry %s does not match image in deployment %s", entry.Image, deploymentEntry.Image)
				status = false
			}
		}
	}
	return status
}

func parseCmdLine() {
	if len(os.Args) != 3 {
		fmt.Print("command line <kubectl cmd> <cluster service version file name>")
		os.Exit(1)
	}
	kubectlCmd = os.Args[1]
	clusterServiceVersionFileName = os.Args[2]
}

func getDeploymentJson() (string, error) {
	cmd := exec.Command(kubectlCmd, "get", "deployments", "-o", "json")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get deployments failed with %w", err)
	}
	return string(out), nil

}

func main() {
	parseCmdLine()

	cluster, cerr := parseClusterServiceVersionFile(clusterServiceVersionFileName)
	if cerr != nil {
		fmt.Println("Parsing of cluster service version failed", cerr)
		os.Exit(1)
	}

	depjson, serr := getDeploymentJson()
	if serr != nil {
		fmt.Println("failed to get deployment", serr)
		os.Exit(1)
	}

	dep, derr := parseDeployments(depjson) //"tools/test-hco-utils/deploy.json")
	if derr != nil {
		fmt.Println("failed to parse the deployment data", derr)
		os.Exit(1)
	}

	if !matchClusterServiceDataToDeployment(cluster, dep) {
		os.Exit(1)
	}
	fmt.Println("*** all deployments are up and corespond to cluster service version ***")
}
