<p>Packages:</p>
<ul>
<li>
<a href="#flux.extensions.gardener.cloud%2fv1alpha1">flux.extensions.gardener.cloud/v1alpha1</a>
</li>
</ul>

<h2 id="flux.extensions.gardener.cloud/v1alpha1">flux.extensions.gardener.cloud/v1alpha1</h2>
<p>

</p>

<h3 id="additionalresource">AdditionalResource
</h3>


<p>
(<em>Appears on:</em><a href="#fluxconfig">FluxConfig</a>)
</p>

<p>
AdditionalResource to sync to the shoot.
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
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name references a resource under Shoot.spec.resources.</p>
</td>
</tr>
<tr>
<td>
<code>targetName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>TargetName optionally overwrites the name of the secret in the shoot.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="fluxconfig">FluxConfig
</h3>


<p>
FluxConfig specifies how to bootstrap Flux on the shoot cluster.
When both "Source" and "Kustomization" are provided they are also installed in the shoot.
Otherwise, only Flux itself is installed with no Objects to reconcile.
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
<code>flux</code></br>
<em>
<a href="#fluxinstallation">FluxInstallation</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Flux configures the Flux installation in the Shoot cluster.</p>
</td>
</tr>
<tr>
<td>
<code>source</code></br>
<em>
<a href="#source">Source</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Source configures how to bootstrap a Flux source object.<br />If provided, a "Kustomization" must also be provided.</p>
</td>
</tr>
<tr>
<td>
<code>kustomization</code></br>
<em>
<a href="#kustomization">Kustomization</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Kustomization configures how to bootstrap a Flux Kustomization object.<br />If provided, "Source" must also be provided.</p>
</td>
</tr>
<tr>
<td>
<code>additionalSecretResources</code></br>
<em>
<a href="#additionalresource">AdditionalResource</a> array
</em>
</td>
<td>
<em>(Optional)</em>
<p>AdditionalSecretResources to sync to the shoot.<br />Secrets referenced here are only created if they don't exist in the shoot yet.<br />When a secret is removed from this list, it is deleted in the shoot.</p>
</td>
</tr>

</tbody>
</table>


<h3 id="fluxinstallation">FluxInstallation
</h3>


<p>
(<em>Appears on:</em><a href="#fluxconfig">FluxConfig</a>)
</p>

<p>
FluxInstallation configures the Flux installation in the Shoot cluster.
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
<code>version</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Version specifies the Flux version that should be installed.<br />Defaults to "v2.8.5".</p>
</td>
</tr>
<tr>
<td>
<code>registry</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Registry specifies the container registry where the Flux controller images are pulled from.<br />Defaults to "ghcr.io/fluxcd".</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Namespace specifes the namespace where Flux should be installed.<br />Defaults to "flux-system".</p>
</td>
</tr>
<tr>
<td>
<code>components</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>Components allows overwriting the components that should be installed.<br />See https://fluxcd.io/flux/installation/configuration/optional-components/ for a list of default<br />components. The minimum required components are: source-controller,kustomize-controller</p>
</td>
</tr>
<tr>
<td>
<code>componentsExtra</code></br>
<em>
string array
</em>
</td>
<td>
<em>(Optional)</em>
<p>ComponentsExtra is a list of extra components to install<br />See https://fluxcd.io/flux/installation/configuration/optional-components/</p>
</td>
</tr>

</tbody>
</table>


<h3 id="kustomization">Kustomization
</h3>


<p>
(<em>Appears on:</em><a href="#fluxconfig">FluxConfig</a>)
</p>

<p>
Kustomization configures how to bootstrap a Flux Kustomization object.
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
<code>template</code></br>
<em>
<a href="https://fluxcd.io/flux/components/kustomize/api/v1/#kustomize.toolkit.fluxcd.io/v1.Kustomization">Kustomization</a>
</em>
</td>
<td>
<p>Template is a partial Kustomization object in API version kustomize.toolkit.fluxcd.io/v1.<br />Required fields: spec.path.<br />The following defaults are applied to omitted field:<br />- metadata.name is defaulted to "flux-system"<br />- metadata.namespace is defaulted to "flux-system"<br />- spec.interval is defaulted to "1m"</p>
</td>
</tr>

</tbody>
</table>


<h3 id="source">Source
</h3>


<p>
(<em>Appears on:</em><a href="#fluxconfig">FluxConfig</a>)
</p>

<p>
Source configures how to bootstrap a Flux source object.
Supported source types: GitRepository, OCIRepository.

The Template field contains a raw Kubernetes object (GitRepository or OCIRepository).
The kind field in the template determines which type is used.

Example GitRepository:

	source:
	  template:
	    apiVersion: source.toolkit.fluxcd.io/v1
	    kind: GitRepository
	    spec:
	      url: https://github.com/example/repo
	      ref:
	        branch: main

Example OCIRepository:

	source:
	  template:
	    apiVersion: source.toolkit.fluxcd.io/v1beta2
	    kind: OCIRepository
	    spec:
	      url: oci://ghcr.io/example/repo
	      ref:
	        tag: latest
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
<code>template</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#rawextension-runtime-pkg">RawExtension</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Template contains a Flux source object (GitRepository or OCIRepository).<br />The kind field determines which type is used.<br />Required fields depend on the source type:<br />- GitRepository: spec.ref.*, spec.url<br />- OCIRepository: spec.ref, spec.url<br />The following defaults are applied to omitted fields:<br />- metadata.name is defaulted to "flux-system"<br />- metadata.namespace is defaulted to "flux-system"<br />- spec.interval is defaulted to "1m"</p>
</td>
</tr>
<tr>
<td>
<code>secretResourceName</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretResourceName references a resource under Shoot.spec.resources.<br />The secret data from this resource is used to create the source's credentials secret<br />(spec.secretRef.name) if specified in Template.</p>
</td>
</tr>

</tbody>
</table>


