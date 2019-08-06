/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2018 Red Hat, Inc.
 *
 */

package main

import (
    "encoding/json"
    "io/ioutil"
    "path"
    "os"

    yaml "github.com/ghodss/yaml"
    csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
    appsv1 "k8s.io/api/apps/v1"
    rbacv1 "k8s.io/api/rbac/v1"

    "github.com/kubevirt/hyperconverged-cluster-operator/tools/util"
)

func main() {

        type csvClusterPermissions struct {
            ServiceAccountName string              `json:"serviceAccountName"`
            Rules              []rbacv1.PolicyRule `json:"rules"`
        }
        type csvDeployments struct {
            Name string                `json:"name"`
            Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
        }
        type csvStrategySpec struct {
            ClusterPermissions []csvClusterPermissions `json:"clusterPermissions"`
            Deployments        []csvDeployments        `json:"deployments"`
        }

        csv_dir := "/csv"

        template_dir := "/template"
        template_name := "kubevirt-hyperconverged-operator.VERSION.clusterserviceversion_merger.yaml.in"
        templateCSVBytes, err := ioutil.ReadFile(path.Join(template_dir, template_name))
        if err != nil {
            panic(err)
        }
        templateStruct := &csvv1.ClusterServiceVersion{}
        err = yaml.Unmarshal(templateCSVBytes, templateStruct)
        if err != nil {
            panic(err)
        }

        files, err := ioutil.ReadDir(csv_dir)
        if err != nil {
           panic(err)
        }

        templateStrategySpec := &csvStrategySpec{}
        json.Unmarshal(templateStruct.Spec.InstallStrategy.StrategySpecRaw, templateStrategySpec)

        for _, file := range files {
            csvBytes, err := ioutil.ReadFile(path.Join(csv_dir, file.Name()))
            if err != nil {
                panic(err)
            }

            csvStruct := &csvv1.ClusterServiceVersion{}

            err = yaml.Unmarshal(csvBytes, csvStruct)
            if err != nil {
                panic(err)
            }

            strategySpec := &csvStrategySpec{}
            json.Unmarshal(csvStruct.Spec.InstallStrategy.StrategySpecRaw, strategySpec)

            deployments := strategySpec.Deployments
            clusterPermissions := strategySpec.ClusterPermissions

            templateStrategySpec.Deployments = append(templateStrategySpec.Deployments, deployments...)
            templateStrategySpec.ClusterPermissions = append(templateStrategySpec.ClusterPermissions, clusterPermissions...)

            for _, owned := range csvStruct.Spec.CustomResourceDefinitions.Owned {
                templateStruct.Spec.CustomResourceDefinitions.Required = append(
                    templateStruct.Spec.CustomResourceDefinitions.Required,
                    csvv1.CRDDescription{
                        Name:       owned.Name,
                        Version:    owned.Version,
                        Kind:       owned.Kind,
                    })
            }
        }

        // Re-serialize deployments and permissions into csv strategy.
        updatedStrat, err := json.Marshal(templateStrategySpec)
        if err != nil {
            panic(err)
        }
        templateStruct.Spec.InstallStrategy.StrategySpecRaw = updatedStrat

        util.MarshallObject(templateStruct, os.Stdout)

}

