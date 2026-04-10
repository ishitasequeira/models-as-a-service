# Subscription limitations and known issues

This document describes known issues and operational considerations for the subscription-based MaaS Platform.

## Subscription Selection Caching

### Cache TTL for Subscription Selection

**Impact:** Medium

**Description:**

Authorino caches the result of the MaaS API subscription selection call (e.g., 60 second TTL). If a user's group membership changes:

- Within the cache window, the old subscription selection may still apply
- After cache expiry, the new group membership is used
- Restarting Authorino pods forces immediate cache invalidation (disruptive)

**Workaround:**

- Wait for cache TTL for changes to fully propagate
- For immediate effect, restart Authorino pods (disruptive; use during maintenance windows)

## API Key vs OpenShift Token

### Group Snapshot in API Keys

**Impact:** Medium

**Description:**

API keys store the user's groups and bound subscription name at creation time. If a user's group membership changes after the key was created:

- The key still carries the **old** groups and subscription until it is revoked and recreated
- Subscription metadata for gateway inference uses the stored groups and subscription from validation
- The user must create a new API key to pick up new groups or a different default subscription

**Workaround:**

- Revoke and recreate API keys when users change groups
- Use OpenShift tokens for interactive use when group membership changes frequently (tokens reflect live group membership)

## Token Rate Limits when Multiple Model References Share One HTTPRoute

**Impact:** High

**Description:**

When more than one **MaaSModelRef** resolves to the **same** **HTTPRoute**, the controller creates multiple **TokenRateLimitPolicy** resources targeting that route. Kuadrant then **enforces only one** of them in practice (others may show **Overridden**; one **Enforced**), so **per-subscription token limits may not all apply** even though CRs look valid.

**Detection:**

Check for multiple TRLPs targeting the same HTTPRoute:

```bash
# List TRLPs that target an HTTPRoute (namespace/name → route name)
kubectl get tokenratelimitpolicy -A -o json | jq -r '.items[] | select(.spec.targetRef.kind=="HTTPRoute") | "\(.metadata.namespace)/\(.metadata.name) → \(.spec.targetRef.name)"' | sort

# Same data plus Enforced condition (needs jq; if this fails, use kubectl describe on each TRLP)
kubectl get tokenratelimitpolicy -A -o json | jq -r '
  .items[] | select(.spec.targetRef.kind == "HTTPRoute")
  | [
      .metadata.namespace,
      .metadata.name,
      .spec.targetRef.name,
      ((.status.conditions // []) | map(select(.type == "Enforced")) | .[0].status // "?")
    ] | @tsv'
```

**How to recognize it:** Several TRLPs share the same `spec.targetRef.name`, with mixed **Overridden** / **Enforced** conditions.

**Workarounds:**

1. **Dedicated routes per model** — Deploy each model with its own HTTPRoute to ensure independent rate limiting
2. **Shared subscription design** — If models must share routes, use a single subscription that covers all models on that route
3. **Route consolidation planning** — Group models by subscription tier on shared routes (e.g., all "premium" models on one route, all "free" models on another)

## Related Documentation

- [Understanding Token Management](token-management.md)
- [Access and Quota Overview](subscription-overview.md)
- [Quota and Access Configuration](quota-and-access-configuration.md)
