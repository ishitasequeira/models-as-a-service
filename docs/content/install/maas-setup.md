# Install MaaS Components

After enabling MaaS in your DataScienceCluster (via `modelsAsService.managementState: Managed`),
the operator will automatically deploy:

- **MaaS API** (Deployment, Service, ServiceAccount, ClusterRole, ClusterRoleBinding, HTTPRoute)
- **MaaS API AuthPolicy** (maas-api-auth-policy) - Protects the MaaS API endpoint
- **NetworkPolicy** (maas-authorino-allow) - Allows Authorino to reach MaaS API

You must manually install the following components:

Provided you have an OpenShift cluster where you had either:

* [installed Open Data Hub project](odh-setup.md);
* or had [installed Red Hat OpenShift AI](rhoai-setup.md)

then you can proceed to install the remaining MaaS components by following this guide.

The tools you will need:

* `kubectl` or `oc` client (this guide uses `kubectl`)
* `kustomize`
* `envsubst`

## Create MaaS Gateway

A Gateway with the name `maas-default-gateway` is **required** for MaaS to function. If you already
created this Gateway during the ODH/RHOAI setup (as documented in [odh-setup.md](odh-setup.md)),
you can skip to the next section.

If the Gateway does not exist yet, create it using the example below:

!!! warning "Example Gateway Configuration"
    The Gateway configuration below is provided as an example. Depending on your cluster setup,
    you may need additional configuration such as TLS certificates, specific listener settings,
    or custom infrastructure labels. Consult your cluster administrator if you're unsure about
    Gateway requirements for your environment.

```shell
export CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')

kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/models-as-a-service.git/deployment/base/networking/maas?ref=main" | \
       envsubst '$CLUSTER_DOMAIN')
```

Wait for the Gateway to be programmed:

```shell
kubectl wait --for=condition=Programmed gateway/maas-default-gateway -n openshift-ingress --timeout=60s
```

## Install Gateway AuthPolicy

Install the authentication policy for the Gateway. This policy applies to model inference traffic
and integrates with the MaaS API for tier-based access control:

```shell
# For RHOAI installations (MaaS API in redhat-ods-applications namespace)
kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/models-as-a-service.git/deployment/base/policies/auth-policies?ref=main" | \
       sed "s/maas-api\.maas-api\.svc/maas-api.redhat-ods-applications.svc/g")

# For ODH installations (MaaS API in opendatahub namespace)
kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/models-as-a-service.git/deployment/base/policies/auth-policies?ref=main" | \
       sed "s/maas-api\.maas-api\.svc/maas-api.opendatahub.svc/g")
```

## Install Usage Policies

Install rate limiting policies (TokenRateLimitPolicy and RateLimitPolicy):

```shell
kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/models-as-a-service.git/deployment/base/policies/usage-policies?ref=main" | \
       envsubst '$CLUSTER_DOMAIN')
```

These policies define:

* **TokenRateLimitPolicy** - Rate limits based on token consumption per tier
* **RateLimitPolicy** - Request rate limits per tier

See [Tier Management](../configuration-and-management/tier-overview.md) for more details on
configuring usage policies and tiers.

## Next steps

* **Deploy models.** In the Quick Start, we provide
  [sample deployments](../quickstart.md#deploy-sample-models-optional) that you
  can use to try the MaaS capability.
* **Perform validation.** Follow the [validation guide](validation.md) to verify that
  MaaS is working correctly.
