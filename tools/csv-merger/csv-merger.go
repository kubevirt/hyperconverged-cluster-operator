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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"sort"
	"strings"

	"dario.cat/mergo"
	"github.com/blang/semver/v4"
	"github.com/ghodss/yaml"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/components"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/util"
)

const (
	operatorName            = "kubevirt-hyperconverged-operator"
	CSVMode                 = "CSV"
	CRDMode                 = "CRDs"
	NPMode                  = "NPs"
	almExamplesAnnotation   = "alm-examples"
	validOutputModes        = CSVMode + "|" + CRDMode + "|" + NPMode
	supported               = "supported"
	operatorFrameworkPrefix = "operatorframework.io/"
	mgImageAnnotation       = "operators.openshift.io/must-gather-image"
	defaultIcon             = "PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA3MDQgNzA3Ij48ZGVmcy8+PGcgZmlsbD0ibm9uZSIgZmlsbC1ydWxlPSJldmVub2RkIj48cGF0aCBkPSJNODguMzMgMTQwLjg5bC4zOC0uNC0uMzguNHpNNzQuMTggMTY3LjcyYy45Ni0zLjMgMi44Ny02LjU4IDUuNzQtOS44Ny0yLjg3IDMuMy00Ljc4IDYuNTgtNS43NCA5Ljg3ek0yMjcuNTIgNjkwLjcxYy0yLjk0IDAtNi42MiAwLTkuNTYtLjk5IDMuNjcgMCA2LjYyLjk5IDkuNTYuOTl6Ii8+PHBhdGggZmlsbD0iIzAwQUFCMiIgZmlsbC1ydWxlPSJub256ZXJvIiBkPSJNNjA2Ljg0IDEzNi45NEwzNzEuMjkgMjAuNTRsLTIuMy0xLjE4YTE4LjUgMTguNSAwIDAwLTQuOTYtMS41OGMtMS41My0uNC0zLjA2LS43OS00LjYtLjc5LTIuMjktLjM5LTQuOTYtLjM5LTcuMjYtLjM5LTQuOTcgMC05LjU2IDAtMTQuNTMuNzktMS41My40LTMuMDYuNC00Ljk3Ljc5TDk3LjEyIDEzNS4zNmEzNC45MSAzNC45MSAwIDAwLTguMDMgNS4xM2wtLjM4LjQgMTIxLjk4IDI1My4zLTkxLjc3LTE5My4zM0gyNzMuOGw2MS45NCAxMTcuOTctMjEuNDEgNDEuMDQtLjc3LjQtNjIuNy0xMTkuOTVIMTgyLjRsMTA3LjgzIDIzNS45NSAxNS4zLTMwYzQuOTYtOS40NiAxNi40NC0xMy40IDI2LTguMjhhMjAuMzMgMjAuMzMgMCAwMTguMDIgMjYuODNsLTI3LjkgNTQuMDYtMjEuNDIgNDEuMDMgNjIuNyAxMjkuODFMNDEyLjIyIDU2OWMtNi4xMiA4LjY4LTE4LjM2IDEwLjY1LTI2Ljc3IDQuMzQtNy42NS01LjUzLTkuOTQtMTYuMTgtNS43NC0yNC40N2wxMy43Ny0yOC40YzUuMzUtOS40NyAxNi44My0xMi42MyAyNi03LjEgOC4wMyA0LjczIDExLjQ3IDE0Ljk5IDguNDEgMjQuMDZsMjcuOTItNTYuODFjLTYuMTIgOC42OC0xOC4zNiAxMC42NS0yNi43NyAzLjk0YTE5LjkzIDE5LjkzIDAgMDEtNS43My0yNC40NmwyNy41My01Ni40MmM0LjU5LTkuODcgMTYuMDYtMTMuODEgMjUuNjItOC42OGExOS42NSAxOS42NSAwIDAxOC40MSAyNi40M2wtNi44OCAxMy44MSAzNS4xOC03MS44MWMtNi4xMiA5LjA4LTE3Ljk4IDExLjQ0LTI2LjM5IDUuMTNhMTkuNzggMTkuNzggMCAwMS02LjUtMjQuODZsMjcuMTUtNTYuNDJjNC41OS05Ljg2IDE2LjA2LTEzLjgxIDI1LjYyLTguNjggOS41NiA0LjczIDEzLjM4IDE2LjU3IDguNDEgMjYuNDRsLTE1LjMgMzEuOTUgNDMuNi04OC43N2MtNS4zNiA5LjQ3LTE2LjgzIDEzLjAyLTI2IDcuNWEyMC4wMyAyMC4wMyAwIDAxLTkuMTgtMTMuMDNoLTIyLjk0Yy0xMC43MSAwLTE5LjEyLTguNjgtMTkuMTItMTkuNzIgMC0xMS4wNSA4LjQxLTE5LjczIDE5LjEyLTE5LjczaDc5LjkxbC0xOS4xMiAzOS4wNiA0Ny4wNC05NS44OGE0MC44MiA0MC44MiAwIDAwLTQuNi0zLjk0IDQxLjg1IDQxLjg1IDAgMDAtOC4wMi01LjUzek00MDUuNyAzNDQuMWwtMjguNjggNTUuNjNjLTQuOTcgOS40Ny0xNi40NCAxMy40Mi0yNiA4LjI5LTkuNTYtNS4xMy0xMy0xNi45Ny04LjAzLTI2LjgzbDI4LjY3LTU1LjY0YzQuOTgtOS40NyAxNi40NS0xMy40MSAyNi04LjI4IDkuNTcgNS4xMyAxMyAxNy4zNiA4LjA0IDI2Ljgzem01OC44OC0xMTUuMjJsLTI4LjY4IDU2LjAzYy00Ljk3IDkuNDctMTYuNDQgMTMuNDItMjYgOC4yOWEyMC4zMyAyMC4zMyAwIDAxLTguMDMtMjYuODNsMzMuNjUtNjYuMjloMTIuMjRjMy4wNiAwIDYuMTIuOCA4LjggMi4zN2ExOS42MiAxOS42MiAwIDAxOC4wMiAyNi40M3oiLz48cGF0aCBmaWxsPSIjRkZGIiBmaWxsLXJ1bGU9Im5vbnplcm8iIGQ9Ik04OS4xIDE0MC41YTkxLjA1IDkxLjA1IDAgMDE4LjAyLTUuMTRMMzMyLjY3IDE4LjE4YzEuNTMtLjQgMy4wNi0uOCA0LjU5LS44IDQuOTctLjc4IDkuOTQtLjc4IDE0LjkxLS43OCAyLjMgMCA0Ljk3IDAgNy4yNy40IDEuNTMuMzkgMy40NC4zOSA0Ljk3Ljc4IDEuNTMuNCAzLjQ0LjggNC45NyAxLjU4bDIuMyAxLjE5IDIzNS41NCAxMTYuNGE0MS44NSA0MS44NSAwIDAxOC4wMyA1LjUyIDQwLjgyIDQwLjgyIDAgMDE0LjU5IDMuOTRsNy4yNy0xNC42Yy0zLjgzLTMuMTUtOC4wMy02LjMxLTEyLjYyLTguNjhoLS43N0wzNzguMTggNi4zNGE1NC4zIDU0LjMgMCAwMC0yNi01LjUyYy03LjY1LS40LTE1LjMuNC0yMi45NSAxLjk3bC0xLjUzLjQtMS41My43OEw5MC42MiAxMjEuMTZhNjYuOTkgNjYuOTkgMCAwMC04Ljc5IDUuNTJsNi44OCAxNC4yLjM4LS4zOXpNNzA1LjUgNDI1LjM3bC0uMzktMS41OC01OC44OS0yNjAuNDF2LTEuMTljLTMuNDQtMTEuNDQtMTAuMzItMjIuMS0xOS41LTI5LjU5bC03LjI2IDE0LjZjNC4yIDQuMzQgOC4wMyA5LjQ3IDEwLjMyIDE1YTIyLjc0IDIyLjc0IDAgMDExLjUzIDQuNzNsNTguNSAyNjAuNDFhOTIgOTIgMCAwMS4zOSAxMC4yNmMwIDMuMTUtLjc3IDUuOTItMS4xNSA5LjA3IDAgLjgtLjM4IDEuNTgtLjM4IDIuMzdhNTYuMjMgNTYuMjMgMCAwMS03LjY1IDE2Ljk3bC03MC4zNiA4OS45Ni05Mi41MyAxMTcuOTdjLTYuMTIgOC42OC0xNS42OCAxNC4yLTI2IDE1Ljc4LTMuMDYuNC02LjUuOC05LjU2LjhIMzUyLjk0bDU4Ljg4IDE1Ljc4aDcwLjc1YzIwLjI2IDAgMzcuMDktOC4yOSA0Ny44LTIyLjg5bDE2Mi44OS0yMDcuOTMuMzgtLjQuMzgtLjRjOS41Ni0xNC42IDEzLjc3LTMxLjk1IDExLjQ3LTQ5LjMxek0yMjIuOTMgNjkwLjEyYy0xLjUzIDAtMy40NC0uNC00Ljk3LS40LTIuMy0uNC00LjItLjc5LTYuNS0xLjU3bC0zLjQ0LTEuMTljLTIuMy0uNzktNC42LTEuOTctNi41LTIuNzZhNjAuMDEgNjAuMDEgMCAwMS05LjE4LTUuOTJjLTEuOTEtMS41OC0zLjgzLTMuMTYtNS4zNi00LjczbC01NC4zLTY5LjQ1LTEwOC4yLTEzOC44OGE1My40MiA1My40MiAwIDAxLTguOC0yMy4yOGMtLjM4LTEuNTgtLjM4LTMuMTYtLjM4LTQuNzQgMC0zLjU1IDAtNi43Ljc2LTEwLjI1bDU4LjEyLTI2MmEyNS42NCAyNS42NCAwIDAxMi4zLTcuMWMyLjY3LTYuNyA2Ljg4LTEyLjIzIDEyLjIzLTE2Ljk2bC02Ljg4LTE0LjJhNTcuNTMgNTcuNTMgMCAwMC0yMi41NiAzNC43MWwtNTguMTIgMjYydi43OWMtMy4wNiAxNy43NSAxLjE0IDM1LjUgMTEuMDkgNTAuMWwuMzguNC4zOC40TDE3NS41MSA2ODMuNGwuMzkuNzkuNzYuNGE2OS44MiA2OS44MiAwIDAwNDUuNSAyMS4zaDEzMC43OHYtMTUuNzhIMjIyLjkzeiIvPjxwYXRoIGZpbGw9IiNGRkYiIGZpbGwtcnVsZT0ibm9uemVybyIgZD0iTTM1Mi45NCA2OTAuMTJ2MTUuNzhoNTguODh6Ii8+PHBhdGggZmlsbD0iIzAwNzk3RiIgZmlsbC1ydWxlPSJub256ZXJvIiBkPSJNMjg5Ljg1IDU2MS4xbC03OS4xNi0xNjYuNUw4OC4zMyAxNDAuODhhNDEuNjggNDEuNjggMCAwMC0xMi4yNCAxNi45NmwtMi4yOSA3LjEtNTcuNzQgMjYyYy0uNzYgMy41NS0uNzYgNi43LS43NiAxMC4yNSAwIDEuNTggMCAzLjE2LjM4IDUuMTNhNTcuNDMgNTcuNDMgMCAwMDguOCAyMy4yOEwxMzMuMDYgNjA0LjVsNTQuMyA2OC42NWMxLjUzIDEuNTggMy40NCAzLjE2IDUuMzUgNC43NGEzNy4wOCAzNy4wOCAwIDAwOS4xOCA1LjkyYzIuMyAxLjE4IDQuMiAxLjk3IDYuNSAyLjc2bDMuNDQgMS4xOGMxLjkxLjc5IDQuMiAxLjE4IDYuNSAxLjU4IDEuNTMuNCAzLjQ0LjQgNC45Ny40aDEzMC4wMUwyOTAuOTkgNTU5LjlsLTEuMTQgMS4xOXoiLz48cGF0aCBkPSJNMTUuMyA0MzcuMmMwLTMuNTUgMC02LjcgMS45LTEwLjI1Ii8+PHBhdGggZmlsbD0iIzAwNzk3RiIgZmlsbC1ydWxlPSJub256ZXJvIiBkPSJNMTk2LjkzIDY4My40MWMtMy40Mi0zLjI5LTYuODMtNi41OC05LjU2LTkuODYgMi43MyAzLjI4IDYuMTQgNi41NyA5LjU2IDkuODZ6Ii8+PHBhdGggZD0iTTIwMi4yOCA2ODcuNzVhNjguNyA2OC43IDAgMDEtOS41Ni05Ljg2TTE4Ny4zNyA2NzMuMTVsLTU0LjMtNjkuMDVNMjE3IDY4OS45MmwtOC42LTIuOTYiLz48cGF0aCBmaWxsPSIjMDA3OTdGIiBmaWxsLXJ1bGU9Im5vbnplcm8iIGQ9Ik0yMTEuNDYgNjkxLjFjLTMuMzgtMS45Ny02Ljc1LTQuOTMtOS41Ni02LjkgMi44IDEuOTcgNi4xOCA0LjkzIDkuNTYgNi45eiIvPjxwYXRoIGZpbGw9IiMzQUNDQzUiIGZpbGwtcnVsZT0ibm9uemVybyIgZD0iTTU3MC4xMyAyNDcuNDJsLTQzLjYgODguNzgtMTEuODQgMjQuNDZhOC42OCA4LjY4IDAgMDEtMS4xNSAxLjk3bC0zNS4xOCA3MS40Mi0yMC42NSA0Mi42MWMtLjM4Ljc5LTEuMTUgMS45Ny0xLjUzIDIuNzZsLTI3LjkxIDU3LjIxYzAgLjQgMCAuNC0uMzkuOGwtMTMuNzYgMjguNGMtLjM4Ljc5LTEuMTUgMS45Ny0xLjUzIDIuNzZsLTU5LjI3IDEyMC43NGgxMjkuNjNjMy4wNiAwIDYuNS0uNCA5LjU2LS43OWEzOS44IDM5LjggMCAwMDI2LTE1Ljc4bDkyLjU0LTExNy45OCA3MC4zNS04OS45NmE1Mi4yIDUyLjIgMCAwMDcuNjUtMTYuOTZjLjM4LS44LjM4LTEuNTguMzgtMi4zN2EzNi45IDM2LjkgMCAwMDEuMTUtOS4wOGMwLTMuNTUgMC02LjctLjM4LTEwLjI1bC01OC41LTI1OS42M2MtLjM5LTEuNTctMS4xNS0zLjE1LTEuNTQtNC43My0yLjI5LTUuNTItNi4xMS0xMC42NS0xMC4zMi0xNWwtNDcuMDMgOTUuNDktMi42OCA1LjEzeiIvPjxwYXRoIGQ9Ik02OTIuMyA0MzcuMmMwIDMuNDMtMS45MSA2LjQ0LTIuODcgOS44N002OTIuNjggNDQ4LjY1Yy0xLjkgMy40NS0zLjgyIDYuOS02LjY5IDkuODZNNDkyLjEyIDY4OS4zM2EzOS44IDM5LjggMCAwMDI2LTE1Ljc4bDkyLjU0LTExNy45OE02OTAuNTggNDI2Ljk1Yy45NiAzLjU1Ljk2IDYuNy45NiAxMC4yNSIvPjxwYXRoIGZpbGw9IiNGRkYiIGZpbGwtcnVsZT0ibm9uemVybyIgZD0iTTM5Ny42OCAzMTcuMjZjLTkuMTgtNS4xMy0yMS4wMy0xLjU4LTI2IDguMjhMMzQzIDM4MS4xOGMtNC45NyA5LjQ3LTEuNTMgMjEuNyA4LjAzIDI2LjgzIDkuMTcgNS4xMyAyMS4wMyAxLjU3IDI2LTguMjlsMjguNjgtNTUuNjNjNC45Ny05LjQ3IDEuMTQtMjEuNy04LjAzLTI2Ljgzek00MTkuMDkgNTExLjM4YTE5LjAzIDE5LjAzIDAgMDAtMjUuNjIgOC42OGwtMTMuNzcgMjguNDFjLTQuNTggOS44Ni0uNzYgMjEuNyA4LjggMjYuNDQgOC40MSA0LjM0IDE4LjM1IDEuNTcgMjMuNy01LjkybDE1LjY4LTMxLjk2YzQuMjEtOS40Ny4zOS0yMC45MS04Ljc5LTI1LjY1ek00MjcuODggNTM3LjgyYzAtLjQgMC0uNC4zOS0uOEw0MTIuNTkgNTY5YTYuNDEgNi40MSAwIDAwMS41My0yLjc2bDEzLjc2LTI4LjQxeiIvPjxwYXRoIGZpbGw9IiNGRkYiIGZpbGwtcnVsZT0ibm9uemVybyIgZD0iTTMxMS42NCA1MTkuMjdsMjcuOTEtNTQuMDVjNC45OC05LjQ3IDEuNTMtMjEuNy04LjAzLTI2LjgzLTkuMTctNS4xMy0yMS4wMy0xLjU4LTI2IDguMjhsLTE1LjMgMjkuOTlMMTgyLjQgMjQwLjMyaDY4LjQ0bDYyLjcxIDExOS45NC43Ny0uNCAyMS4wMy00MC42My02MS45NS0xMTguMzdIMTE4LjU0TDIxMC4zIDM5NC4ybDc5LjkyIDE2NS43MSAyMS40MS00MC42NHoiLz48cGF0aCBmaWxsPSIjRkZGIiBmaWxsLXJ1bGU9Im5vbnplcm8iIGQ9Ik0yOTAuMjMgNTYwLjMxbC03OS41NC0xNjUuNzIgNzkuMTYgMTY2LjUxek01OTEuNTQgMjAzLjIzaC03OS45MWMtMTAuNzEgMC0xOS4xMiA4LjY4LTE5LjEyIDE5LjczIDAgMTEuMDQgOC40MSAxOS43MiAxOS4xMiAxOS43MmgyMi45NGMyLjMgMTAuNjYgMTIuNjIgMTcuMzcgMjIuOTQgMTVhMTkuNSAxOS41IDAgMDAxMi42Mi05LjQ3bDIuNjgtNS41MyAxOC43My0zOS40NXpNNTc2LjgyIDI0Mi4yOWwtNi42OSA5Ljg2Ljk2LS43ek01NDEuODMgMzA0LjYzYzQuOTgtOS44NiAxLjE1LTIxLjctOC40LTI2LjQ0LTkuNTctNC43My0yMS4wNC0xLjE4LTI1LjYzIDguNjl2LjM5bC0yNy4xNSA1Ni40MmMtNC41OSA5Ljg3LS43NiAyMS43IDguOCAyNi40NCA4LjQxIDQuMzQgMTguNzMgMS41OCAyNC4wOS02LjcxbDEzLTI2LjgzIDE1LjMtMzEuOTZ6Ii8+PHBhdGggZmlsbD0iI0ZGRiIgZmlsbC1ydWxlPSJub256ZXJvIiBkPSJNNTI2LjU0IDMzNi41OWwtMTMgMjYuODNjLjM4LS43OS43Ni0xLjU4IDEuMTUtMS45N2wxMS44NS0yNC44NnpNNDg0Ljg2IDQyMS4wM2M0LjU5LTkuODcuNzYtMjEuNy04LjQxLTI2LjQ0LTkuMTgtNC43My0yMS4wMy0uNzktMjUuNjIgOC42OGwtMjcuMTUgNTYuNDJjLTQuNTkgOS44Ny0uNzcgMjEuNyA4LjggMjYuNDQgOC40IDQuMzQgMTguMzUgMS41OCAyMy43LTUuOTJsMjIuMTgtNDUuMzcgNi41LTEzLjgxeiIvPjxwYXRoIGZpbGw9IiNGRkYiIGZpbGwtcnVsZT0ibm9uemVybyIgZD0iTTQ3OC4zNiA0MzQuODRsLTIyLjE4IDQ1LjM3Yy43Ny0uNzkgMS4xNS0xLjk3IDEuNTMtMi43NmwyMC42NS00Mi42MXpNNDU2LjU2IDIwMi40NGExNy4zNCAxNy4zNCAwIDAwLTguOC0yLjM3aC0xMS44NWwtMzMuNjQgNjYuMjlhMjAuMTUgMjAuMTUgMCAwMDQuOTcgMjcuNjJjOC44IDYuMzEgMjAuNjQgMy45NCAyNi43Ni01LjEzLjc3LTEuMTkgMS41My0yLjM3IDEuOTEtMy45NWwyOC42OC01NS42M2M0Ljk3LTkuODYgMS41My0yMS43LTguMDMtMjYuODMuMzkgMCAwIDAgMCAweiIvPjwvZz48L3N2Zz4="
)

var (
	supportedArches = []string{"arch.amd64"}
	supportedOS     = []string{"os.linux"}
)

type EnvVarFlags []corev1.EnvVar

func (i *EnvVarFlags) String() string {
	es := make([]string, 0)
	for _, ev := range *i {
		es = append(es, fmt.Sprintf("%s=%s", ev.Name, ev.Value))
	}
	return strings.Join(es, ",")
}

func (i *EnvVarFlags) Set(value string) error {
	kv := strings.Split(value, "=")
	*i = append(*i, corev1.EnvVar{
		Name:  kv[0],
		Value: kv[1],
	})
	return nil
}

var (
	outputMode          = flag.String("output-mode", CSVMode, "Working mode: "+validOutputModes)
	cnaCsv              = flag.String("cna-csv", "", "Cluster Network Addons CSV string")
	virtCsv             = flag.String("virt-csv", "", "KubeVirt CSV string")
	sspCsv              = flag.String("ssp-csv", "", "Scheduling Scale Performance CSV string")
	cdiCsv              = flag.String("cdi-csv", "", "Containerized Data Importer CSV String")
	hppCsv              = flag.String("hpp-csv", "", "HostPath Provisioner Operator CSV String")
	aaqCsv              = flag.String("aaq-csv", "", "Applications Aware Quota Operator CSV String")
	operatorImage       = flag.String("operator-image-name", "", "HyperConverged Cluster Operator image")
	webhookImage        = flag.String("webhook-image-name", "", "HyperConverged Cluster Webhook image")
	cliDownloadsImage   = flag.String("cli-downloads-image-name", "", "Downloads Server image")
	kvUIPluginImage     = flag.String("kubevirt-consoleplugin-image-name", "", "KubeVirt Console Plugin image")
	kvUIProxyImage      = flag.String("kubevirt-consoleproxy-image-name", "", "KubeVirt Console Proxy image")
	kvVirtIOWinImage    = flag.String("kv-virtiowin-image-name", "", "KubeVirt VirtIO Win image")
	passtImage          = flag.String("network-passt-binding-image-name", "", "Passt binding image")
	passtCNIImage       = flag.String("network-passt-binding-cni-image-name", "", "Passt binding cni image")
	waspAgentImage      = flag.String("wasp-agent-image-name", "", "Wasp Agent image")
	smbios              = flag.String("smbios", "", "Custom SMBIOS string for KubeVirt ConfigMap")
	machinetype         = flag.String("machinetype", "", "Custom MACHINETYPE string for KubeVirt ConfigMap (Deprecated, use amd64-machinetype)")
	amd64MachineType    = flag.String("amd64-machinetype", "", "Custom AMD64_MACHINETYPE string for KubeVirt ConfigMap")
	arm64MachineType    = flag.String("arm64-machinetype", "", "Custom ARM64_MACHINETYPE string for KubeVirt ConfigMap")
	csvVersion          = flag.String("csv-version", "", "CSV version")
	replacesCsvVersion  = flag.String("replaces-csv-version", "", "CSV version to replace")
	metadataDescription = flag.String("metadata-description", "", "One-Liner Description")
	specDescription     = flag.String("spec-description", "", "Description")
	specDisplayName     = flag.String("spec-displayname", "", "Display Name")
	icon                = flag.String("icon", defaultIcon, "The project logo, as base64 text, in image/svg+xml format")
	namespace           = flag.String("namespace", "kubevirt-hyperconverged", "Namespace")
	crdDisplay          = flag.String("crd-display", "KubeVirt HyperConverged Cluster", "Label show in OLM UI about the primary CRD")
	csvOverrides        = flag.String("csv-overrides", "", "CSV like string with punctual changes that will be recursively applied (if possible)")
	visibleCRDList      = flag.String("visible-crds-list", "hyperconvergeds.hco.kubevirt.io,hostpathprovisioners.hostpathprovisioner.kubevirt.io",
		"Comma separated list of all the CRDs that should be visible in OLM console")
	relatedImagesList = flag.String("related-images-list", "",
		"Comma separated list of all the images referred in the CSV (just the image pull URLs or eventually a set of 'image|name' collations)")
	ignoreComponentsRelatedImages = flag.Bool("ignore-component-related-image", false, "Ignore relatedImages from components CSVs")
	hcoKvIoVersion                = flag.String("hco-kv-io-version", "", "KubeVirt version")
	kubevirtVersion               = flag.String("kubevirt-version", "", "Kubevirt operator version")
	kvVirtLauncherOSVersion       = flag.String("virt-launcher-os-version", "", "Virt launcher OS version")
	cdiVersion                    = flag.String("cdi-version", "", "CDI operator version")
	cnaoVersion                   = flag.String("cnao-version", "", "CNA operator version")
	sspVersion                    = flag.String("ssp-version", "", "SSP operator version")
	hppoVersion                   = flag.String("hppo-version", "", "HPP operator version")
	aaqVersion                    = flag.String("aaq-version", "", "AAQ operator version")
	enableUniqueSemver            = flag.Bool("enable-unique-version", false, "Insert a skipRange annotation to support unique semver in the CSV")
	skipsList                     = flag.String("skips-list", "",
		"Comma separated list of CSVs that can be skipped (read replaced) by this version")
	olmSkipRange = flag.String("olm-skip-range", "",
		"Semver range expression for CSVs that can be skipped (read replaced) by this version")
	mgImage             = flag.String("mg-image", "quay.io/kubevirt/must-gather", "Operator suggested must-gather image")
	testImagesNVRs      = flag.String("test-images-nvrs", "", "Test Images NVRs")
	dumpNetworkPolicies = flag.Bool("dump-network-policies", false, "Dump network policy yamls to stdout")

	envVars EnvVarFlags
)

func main() {
	flag.Var(&envVars, "env-var", "HCO environment variable (key=value), may be used multiple times")

	flag.Parse()

	if webhookImage == nil || *webhookImage == "" {
		*webhookImage = *operatorImage
	}

	if *enableUniqueSemver && *olmSkipRange != "" {
		panicOnError(errors.New("enable-unique-version and olm-skip-range cannot be used and the same time"))
		os.Exit(1)
	}

	switch *outputMode {
	case CRDMode:
		_, err := os.Stdout.Write(crdBytes)
		panicOnError(err)
	case CSVMode:
		getHcoCsv()
	case NPMode:
		*dumpNetworkPolicies = true
	default:
		panic("Unsupported output mode: " + *outputMode)
	}

	if *dumpNetworkPolicies {
		panicOnError(generateNetworkPolicies())
	}
}

func getHcoCsv() {
	if *specDisplayName == "" || *specDescription == "" {
		panic(errors.New("must specify spec-displayname and spec-description"))
	}

	componentsWithCsvs := getInitialCsvList()

	version := semver.MustParse(*csvVersion)
	replaces := getReplacesVersion()

	csvParams := getCsvBaseParams(version)

	// This is the basic CSV without an InstallStrategy defined
	csvBase := components.GetCSVBase(csvParams)

	// Only set the Replaces field if a replaces version was provided
	if replaces != "" {
		csvBase.Spec.Replaces = replaces
	}

	if *enableUniqueSemver {
		csvBase.Annotations["olm.skipRange"] = fmt.Sprintf("<%v", version.String())
	} else if *olmSkipRange != "" {
		csvBase.Annotations["olm.skipRange"] = *olmSkipRange
	}

	params := getDeploymentParams()
	// This is the base deployment + rbac for the HCO CSV
	installStrategyBase := components.GetInstallStrategyBase(params)

	overwriteDeploymentSpecLabels(installStrategyBase.DeploymentSpecs, hcoutil.AppComponentDeployment)

	relatedImages := getRelatedImages()

	processCsvs(componentsWithCsvs, installStrategyBase, csvBase, &relatedImages)

	csvBase.Spec.RelatedImages = relatedImages

	if *skipsList != "" {
		csvBase.Spec.Skips = strings.Split(*skipsList, ",")
	}

	hiddenCRDsJ, err := getHiddenCrds(*csvBase)
	panicOnError(err)

	csvBase.Annotations["operators.operatorframework.io/internal-objects"] = hiddenCRDsJ

	// Update csv strategy.
	csvBase.Spec.InstallStrategy.StrategyName = "deployment"
	csvBase.Spec.InstallStrategy.StrategySpec = *installStrategyBase

	if *metadataDescription != "" {
		csvBase.Annotations["description"] = *metadataDescription
	}
	if *specDescription != "" {
		csvBase.Spec.Description = *specDescription
	}
	if *specDisplayName != "" {
		csvBase.Spec.DisplayName = *specDisplayName
	}
	if *mgImage != "" {
		csvBase.Annotations[mgImageAnnotation] = *mgImage
	}
	if *testImagesNVRs != "" {
		csvBase.Annotations["test-images-nvrs"] = *testImagesNVRs
	}

	setSupported(csvBase)

	applyOverrides(csvBase)

	csvBase.Spec.RelatedImages = sortRelatedImages(csvBase.Spec.RelatedImages)
	panicOnError(util.MarshallObject(csvBase, os.Stdout))
}

func getHiddenCrds(csvBase csvv1alpha1.ClusterServiceVersion) (string, error) {
	hiddenCrds := make([]string, 0)
	visibleCrds := strings.Split(*visibleCRDList, ",")
	for _, owned := range csvBase.Spec.CustomResourceDefinitions.Owned {
		if !slices.Contains(visibleCrds, owned.Name) {
			hiddenCrds = append(
				hiddenCrds,
				owned.Name,
			)
		}
	}

	hiddenCrdsJ, err := json.Marshal(hiddenCrds)
	if err != nil {
		return "", err
	}
	return string(hiddenCrdsJ), nil
}

func processCsvs(componentsWithCsvs []util.CsvWithComponent, installStrategyBase *csvv1alpha1.StrategyDetailsDeployment, csvBase *csvv1alpha1.ClusterServiceVersion, ris *[]csvv1alpha1.RelatedImage) {
	for _, c := range componentsWithCsvs {
		processOneCsv(c, installStrategyBase, csvBase, ris)
	}
}

func processOneCsv(c util.CsvWithComponent, installStrategyBase *csvv1alpha1.StrategyDetailsDeployment, csvBase *csvv1alpha1.ClusterServiceVersion, ris *[]csvv1alpha1.RelatedImage) {
	if c.Csv == "" {
		log.Panicf("ERROR: the %s CSV was empty", c.Name)
	}
	csvBytes := []byte(c.Csv)

	csvStruct := &csvv1alpha1.ClusterServiceVersion{}

	panicOnError(yaml.Unmarshal(csvBytes, csvStruct), "failed to unmarshal the CSV for", c.Name)

	strategySpec := csvStruct.Spec.InstallStrategy.StrategySpec

	overwriteDeploymentSpecLabels(strategySpec.DeploymentSpecs, c.Component)
	installStrategyBase.DeploymentSpecs = append(installStrategyBase.DeploymentSpecs, strategySpec.DeploymentSpecs...)

	installStrategyBase.ClusterPermissions = append(installStrategyBase.ClusterPermissions, strategySpec.ClusterPermissions...)
	installStrategyBase.Permissions = append(installStrategyBase.Permissions, strategySpec.Permissions...)

	csvBase.Spec.WebhookDefinitions = append(csvBase.Spec.WebhookDefinitions, csvStruct.Spec.WebhookDefinitions...)

	for _, owned := range csvStruct.Spec.CustomResourceDefinitions.Owned {
		csvBase.Spec.CustomResourceDefinitions.Owned = append(
			csvBase.Spec.CustomResourceDefinitions.Owned,
			newCRDDescription(owned),
		)
	}
	csvBaseAlmString := csvBase.Annotations[almExamplesAnnotation]
	csvStructAlmString := csvStruct.Annotations[almExamplesAnnotation]
	var baseAlmcrs []interface{}
	var structAlmcrs []interface{}

	if !strings.HasPrefix(csvBaseAlmString, "[") {
		csvBaseAlmString = "[" + csvBaseAlmString + "]"
	}

	panicOnError(json.Unmarshal([]byte(csvBaseAlmString), &baseAlmcrs), "failed to unmarshal the example from base from base csv for", c.Name, "csvBaseAlmString:", csvBaseAlmString)
	panicOnError(json.Unmarshal([]byte(csvStructAlmString), &structAlmcrs), "failed to unmarshal the example from base from struct csv for", c.Name, "csvStructAlmString:", csvStructAlmString)

	baseAlmcrs = append(baseAlmcrs, structAlmcrs...)
	almB, err := json.Marshal(baseAlmcrs)
	panicOnError(err, "failed to marshal the combined example for", c.Name)
	csvBase.Annotations[almExamplesAnnotation] = string(almB)

	if !*ignoreComponentsRelatedImages {
		for _, image := range csvStruct.Spec.RelatedImages {
			*ris = appendRelatedImageIfMissing(*ris, image)
		}
	}
}

func newCRDDescription(owned csvv1alpha1.CRDDescription) csvv1alpha1.CRDDescription {
	return csvv1alpha1.CRDDescription{
		Name:        owned.Name,
		Version:     owned.Version,
		Kind:        owned.Kind,
		Description: owned.Description,
		DisplayName: owned.DisplayName,
	}
}

func applyOverrides(csvBase *csvv1alpha1.ClusterServiceVersion) {
	if *csvOverrides != "" {
		csvOBytes := []byte(*csvOverrides)

		csvO := &csvv1alpha1.ClusterServiceVersion{}

		panicOnError(yaml.Unmarshal(csvOBytes, csvO))

		panicOnError(mergo.Merge(csvBase, csvO, mergo.WithOverride))
	}
}

func setSupported(csvBase *csvv1alpha1.ClusterServiceVersion) {
	if csvBase.Labels == nil {
		csvBase.Labels = make(map[string]string)
	}
	for _, ele := range supportedArches {
		csvBase.Labels[operatorFrameworkPrefix+ele] = supported
	}
	for _, ele := range supportedOS {
		csvBase.Labels[operatorFrameworkPrefix+ele] = supported
	}
}

func getInitialCsvList() []util.CsvWithComponent {
	return []util.CsvWithComponent{
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
			Name:      "CDI",
			Csv:       *cdiCsv,
			Component: hcoutil.AppComponentStorage,
		},
		{
			Name:      "HPP",
			Csv:       *hppCsv,
			Component: hcoutil.AppComponentStorage,
		},
		{
			Name:      "AAQ",
			Csv:       *aaqCsv,
			Component: hcoutil.AppComponentQuotaMngt,
		},
	}
}

func getReplacesVersion() string {
	if *replacesCsvVersion != "" {
		return fmt.Sprintf("%v.v%v", operatorName, semver.MustParse(*replacesCsvVersion).String())
	}
	return ""
}

func getRelatedImages() []csvv1alpha1.RelatedImage {
	var ris []csvv1alpha1.RelatedImage

	for _, image := range strings.Split(*relatedImagesList, ",") {
		if image != "" {
			ris = addRelatedImage(ris, image)
		}
	}
	return ris
}

func getCsvBaseParams(version semver.Version) *components.CSVBaseParams {
	return &components.CSVBaseParams{
		Name:            operatorName,
		Namespace:       *namespace,
		DisplayName:     *specDisplayName,
		MetaDescription: *metadataDescription,
		Description:     *specDescription,
		Image:           *operatorImage,
		Version:         version,
		CrdDisplay:      *crdDisplay,
		Icon:            *icon,
	}
}

func getDeploymentParams() *components.DeploymentOperatorParams {
	return &components.DeploymentOperatorParams{
		Namespace:              *namespace,
		Image:                  *operatorImage,
		WebhookImage:           *webhookImage,
		CliDownloadsImage:      *cliDownloadsImage,
		KVUIPluginImage:        *kvUIPluginImage,
		KVUIProxyImage:         *kvUIProxyImage,
		ImagePullPolicy:        "IfNotPresent",
		VirtIOWinContainer:     *kvVirtIOWinImage,
		Smbios:                 *smbios,
		Machinetype:            *machinetype,
		Amd64MachineType:       *amd64MachineType,
		Arm64MachineType:       *arm64MachineType,
		HcoKvIoVersion:         *hcoKvIoVersion,
		KubevirtVersion:        *kubevirtVersion,
		KvVirtLancherOsVersion: *kvVirtLauncherOSVersion,
		CdiVersion:             *cdiVersion,
		CnaoVersion:            *cnaoVersion,
		SspVersion:             *sspVersion,
		HppoVersion:            *hppoVersion,
		AaqVersion:             *aaqVersion,
		PasstImage:             *passtImage,
		PasstCNIImage:          *passtCNIImage,
		WaspAgentImage:         *waspAgentImage,
		Env:                    envVars,
		AddNetworkPolicyLabels: *dumpNetworkPolicies,
	}
}

func overwriteDeploymentSpecLabels(specs []csvv1alpha1.StrategyDeploymentSpec, component hcoutil.AppComponent) {
	for i := range specs {
		if specs[i].Label == nil {
			specs[i].Label = make(map[string]string)
		}
		if specs[i].Spec.Template.Labels == nil {
			specs[i].Spec.Template.Labels = make(map[string]string)
		}
		overwriteWithStandardLabels(specs[i].Spec.Template.Labels, *hcoKvIoVersion, component)
		overwriteWithStandardLabels(specs[i].Label, *hcoKvIoVersion, component)
	}

}

func overwriteWithStandardLabels(labels map[string]string, version string, component hcoutil.AppComponent) {
	labels[hcoutil.AppLabelManagedBy] = "olm"
	labels[hcoutil.AppLabelVersion] = version
	labels[hcoutil.AppLabelPartOf] = hcoutil.HyperConvergedCluster
	labels[hcoutil.AppLabelComponent] = string(component)
}

// add image to the slice. Ignore if the image already exists in the slice
func addRelatedImage(images []csvv1alpha1.RelatedImage, image string) []csvv1alpha1.RelatedImage {
	var ri csvv1alpha1.RelatedImage
	if strings.Contains(image, "|") {
		imageS := strings.Split(image, "|")
		ri.Image = imageS[0]
		ri.Name = imageS[1]
	} else {
		names := strings.Split(strings.Split(image, "@")[0], "/")
		ri.Name = names[len(names)-1]
		ri.Image = image
	}

	return appendRelatedImageIfMissing(images, ri)
}

func panicOnError(err error, info ...string) {
	if err != nil {
		moreInfo := strings.Join(info, " ")

		log.Println("Error!", err, moreInfo)
		panic(err)
	}
}

func appendRelatedImageIfMissing(slice []csvv1alpha1.RelatedImage, ri csvv1alpha1.RelatedImage) []csvv1alpha1.RelatedImage {
	for _, ele := range slice {
		if ele.Image == ri.Image {
			return slice
		}
	}
	return append(slice, ri)
}

func sortRelatedImages(slice []csvv1alpha1.RelatedImage) []csvv1alpha1.RelatedImage {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].Name < slice[j].Name
	})
	return slice
}
