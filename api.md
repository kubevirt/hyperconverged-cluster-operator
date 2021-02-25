<p>Packages:</p>
<ul>
<li>
<a href="#hco.kubevirt.io%2fv1beta1">hco.kubevirt.io/v1beta1</a>
</li>
</ul>
<h2 id="hco.kubevirt.io/v1beta1">hco.kubevirt.io/v1beta1</h2>
<p>
<p>package v1beta1 contains API Schema definitions for the hco v1beta1 API group</p>
</p>
Resource Types:
<ul></ul>
<h3 id="hco.kubevirt.io/v1beta1.HyperConverged">HyperConverged
</h3>
<p>
<p>HyperConverged is the Schema for the hyperconvergeds API</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedSpec">
HyperConvergedSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>localStorageClassName</code><br/>
<em>
string
</em>
</td>
<td>
<p>LocalStorageClassName the name of the local storage class.</p>
</td>
</tr>
<tr>
<td>
<code>infra</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedConfig">
HyperConvergedConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>infra HyperConvergedConfig influences the pod configuration (currently only placement)
for all the infra components needed on the virtualization enabled cluster
but not necessarely directly on each node running VMs/VMIs.</p>
</td>
</tr>
<tr>
<td>
<code>workloads</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedConfig">
HyperConvergedConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>workloads HyperConvergedConfig influences the pod configuration (currently only placement) of components
which need to be running on a node where virtualization workloads should be able to run.
Changes to Workloads HyperConvergedConfig can be applied only without existing workload.</p>
</td>
</tr>
<tr>
<td>
<code>featureGates</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedFeatureGates">
HyperConvergedFeatureGates
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>featureGates is a map of feature gate flags. Setting a flag to <code>true</code> will enable
the feature. Setting <code>false</code> or removing the feature gate, disables the feature.</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<p>operator version</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedStatus">
HyperConvergedStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="hco.kubevirt.io/v1beta1.HyperConvergedConfig">HyperConvergedConfig
</h3>
<p>
(<em>Appears on:</em><a href="#hco.kubevirt.io/v1beta1.HyperConvergedSpec">HyperConvergedSpec</a>)
</p>
<p>
<p>HyperConvergedConfig defines a set of configurations to pass to components</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodePlacement</code><br/>
<em>
kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api.NodePlacement
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodePlacement describes node scheduling configuration.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="hco.kubevirt.io/v1beta1.HyperConvergedFeatureGates">HyperConvergedFeatureGates
</h3>
<p>
(<em>Appears on:</em><a href="#hco.kubevirt.io/v1beta1.HyperConvergedSpec">HyperConvergedSpec</a>)
</p>
<p>
<p>HyperConvergedFeatureGates is a set of optional feature gates to enable or disable new features that are not enabled
by default yet.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sriovLiveMigration</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allow migrating a virtual machine with SRIOV interfaces.
When enabled virt-launcher pods of virtual machines with SRIOV
interfaces run with CAP_SYS_RESOURCE capability.
This may degrade virt-launcher security.</p>
</td>
</tr>
<tr>
<td>
<code>hotplugVolumes</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allow attaching a data volume to a running VMI</p>
</td>
</tr>
<tr>
<td>
<code>gpu</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allow assigning GPU and vGPU devices to virtual machines</p>
</td>
</tr>
<tr>
<td>
<code>hostDevices</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allow assigning host devices to virtual machines</p>
</td>
</tr>
<tr>
<td>
<code>withHostPassthroughCPU</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Allow migrating a virtual machine with CPU host-passthrough mode. This should be
enabled only when the Cluster is homogeneous from CPU HW perspective doc here</p>
</td>
</tr>
<tr>
<td>
<code>withHostModelCPU</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Support migration for VMs with host-model CPU mode</p>
</td>
</tr>
<tr>
<td>
<code>hypervStrictCheck</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enable HyperV strict host checking for HyperV enlightenments
Defaults to true, even when HyperConvergedFeatureGates is empty</p>
</td>
</tr>
</tbody>
</table>
<h3 id="hco.kubevirt.io/v1beta1.HyperConvergedSpec">HyperConvergedSpec
</h3>
<p>
(<em>Appears on:</em><a href="#hco.kubevirt.io/v1beta1.HyperConverged">HyperConverged</a>)
</p>
<p>
<p>HyperConvergedSpec defines the desired state of HyperConverged</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>localStorageClassName</code><br/>
<em>
string
</em>
</td>
<td>
<p>LocalStorageClassName the name of the local storage class.</p>
</td>
</tr>
<tr>
<td>
<code>infra</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedConfig">
HyperConvergedConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>infra HyperConvergedConfig influences the pod configuration (currently only placement)
for all the infra components needed on the virtualization enabled cluster
but not necessarely directly on each node running VMs/VMIs.</p>
</td>
</tr>
<tr>
<td>
<code>workloads</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedConfig">
HyperConvergedConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>workloads HyperConvergedConfig influences the pod configuration (currently only placement) of components
which need to be running on a node where virtualization workloads should be able to run.
Changes to Workloads HyperConvergedConfig can be applied only without existing workload.</p>
</td>
</tr>
<tr>
<td>
<code>featureGates</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.HyperConvergedFeatureGates">
HyperConvergedFeatureGates
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>featureGates is a map of feature gate flags. Setting a flag to <code>true</code> will enable
the feature. Setting <code>false</code> or removing the feature gate, disables the feature.</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<p>operator version</p>
</td>
</tr>
</tbody>
</table>
<h3 id="hco.kubevirt.io/v1beta1.HyperConvergedStatus">HyperConvergedStatus
</h3>
<p>
(<em>Appears on:</em><a href="#hco.kubevirt.io/v1beta1.HyperConverged">HyperConverged</a>)
</p>
<p>
<p>HyperConvergedStatus defines the observed state of HyperConverged</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br/>
<em>
[]github.com/openshift/custom-resource-status/conditions/v1.Condition
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions describes the state of the HyperConverged resource.</p>
</td>
</tr>
<tr>
<td>
<code>relatedObjects</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectreference-v1-core">
[]Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>RelatedObjects is a list of objects created and maintained by this
operator. Object references will be added to this list after they have
been created AND found in the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>versions</code><br/>
<em>
<a href="#hco.kubevirt.io/v1beta1.Versions">
Versions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Versions is a list of HCO component versions, as name/version pairs. The version with a name of &ldquo;operator&rdquo;
is the HCO version itself, as described here:
<a href="https://github.com/openshift/cluster-version-operator/blob/master/docs/dev/clusteroperator.md#version">https://github.com/openshift/cluster-version-operator/blob/master/docs/dev/clusteroperator.md#version</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="hco.kubevirt.io/v1beta1.Version">Version
</h3>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="hco.kubevirt.io/v1beta1.Versions">Versions
(<code>[]github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1beta1.Version</code> alias)</p></h3>
<p>
(<em>Appears on:</em><a href="#hco.kubevirt.io/v1beta1.HyperConvergedStatus">HyperConvergedStatus</a>)
</p>
<p>
</p>
<h3 id="hco.kubevirt.io/v1beta1.WebhookHandlerIfs">WebhookHandlerIfs
</h3>
<p>
</p>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>8e08d42</code>.
</em></p>
