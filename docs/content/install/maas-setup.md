# Install Models-as-a-Service

The Models-as-a-Service (MaaS) capability can be installed in two ways:

**If using operator-based installation** (MaaS enabled in DataScienceCluster):

The operator automatically deploys the MaaS API, AuthPolicy, and NetworkPolicy. You only need to 
[install the usage policies](#install-usage-policies-operator-based) manually.

**If using standalone Kustomize installation**:

Follow the [full Kustomize installation](#install-maas-using-the-kustomize-manifest) below.

---

Provided you have an OpenShift cluster where you had either:

* [installed Open Data Hub project](odh-setup.md);
* or had [installed Red Hat OpenShift AI](rhoai-setup.md)

then you can proceed to install MaaS capabilities by following this guide.

The tools you will need:

* `kubectl` or `oc` client (this guide uses `kubectl`)
* `kustomize`
* `jq`
* `envsubst`
* `base64`
* `cut`
  
## Install MaaS using the Kustomize manifest

Install MaaS by running the following commands:

```shell
export CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')

kubectl create namespace maas-api
kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/models-as-a-service.git/deployment/overlays/openshift?ref=main" | \
       envsubst '$CLUSTER_DOMAIN')
```

The Kustomize manifest will:

* Create a Gateway as the ingress point for any traffic related to MaaS (for inference 
  and for the MaaS API).
* Install the support MaaS API (`Deployment`, `Service`, `ServiceAccount`, 
  `ClusterRole`, `ClusterRoleBinding`, `HTTPRoute`, `AuthPolicy`, and `NetworkPolicy`).
* Install predefined policies: authentication, authorization and rate limits (See 
  [Tier Management](../configuration-and-management/tier-overview.md))

### Policy audience adjustment

The default audience of Kubernetes clusters is usually `https://kubernetes.default.svc`.
You can check the audience of your cluster with the following commands:

```shell
AUD="$(kubectl create token default --duration=10m 2>/dev/null | cut -d. -f2 | jq -Rr '@base64d | fromjson | .aud[0]' 2>/dev/null)"
echo $AUD

# Output:
#   https://kubernetes.default.svc
```

The Kustomize manifest uses the default audience for the installed MaaS API policy. If 
the output of the previous script is different from a non-empty string and 
`https://kubernetes.default.svc`, you are required to patch the policy of the MaaS API:

```shell
kubectl patch authpolicy maas-api-auth-policy -n maas-api --type=merge --patch-file <(echo "  
spec:
  rules:
    authentication:
      openshift-identities:
        kubernetesTokenReview:
          audiences:
            - $AUD
            - maas-default-gateway-sa")
```

## Install Usage Policies (Operator-based)

If you enabled MaaS via the DataScienceCluster (`modelsAsService.managementState: Managed`),
the operator has already installed the MaaS API and AuthPolicy. You only need to install 
the usage policies (TokenRateLimitPolicy and RateLimitPolicy):

```shell
export CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')

kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/models-as-a-service.git/deployment/base/policies/usage-policies?ref=main" | \
       envsubst '$CLUSTER_DOMAIN')
```

Optionally, install the MaaS Gateway if not using an existing gateway:

```shell
kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/models-as-a-service.git/deployment/base/networking/maas?ref=main" | \
       envsubst '$CLUSTER_DOMAIN')
```

## Next steps

* **Deploy models.** In the Quick Start, we provide
  [sample deployments](../quickstart.md#deploy-sample-models-optional) that you 
  can use to try the MaaS capability.
* **Perform validation.** Follow the [validation guide](validation.md) to verify that 
  MaaS is working correctly.
