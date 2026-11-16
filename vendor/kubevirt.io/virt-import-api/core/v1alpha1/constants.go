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
 * Copyright The KubeVirt Authors.
 *
 */

package v1alpha1

import importapi "kubevirt.io/virt-import-api/core"

const (
	// FinalizerImportCleanup guards cleanup of the intermediate resources
	// created during an import (metadata-fetch Job, DataVolumes, PVCs).
	FinalizerImportCleanup = importapi.GroupName + "/ImportCleanup"

	// LabelImportUID marks resources created for a VirtualMachineImport,
	// and scopes the controller's informer caches to them.
	LabelImportUID = importapi.GroupName + "/ImportUID"

	ConditionReady       = "Ready"
	ConditionProgressing = "Progressing"

	// Reasons for the Progressing and Ready conditions.
	ReasonFetchingMetadata = "FetchingMetadata"
	ReasonImportingDisks   = "ImportingDisks"
	ReasonCreatingTarget   = "CreatingTarget"
	ReasonImportComplete   = "ImportComplete"
	ReasonImportFailed     = "ImportFailed"
	ReasonWaiting          = "Waiting"
)
