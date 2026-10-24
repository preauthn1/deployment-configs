# dbc-gateway Kubernetes deployment

This directory is a non-production deployment template. Review every placeholder before applying it.

## Required secrets

Create the Google credential Secret from the service account JSON file. Do not commit the JSON file:

```bash
kubectl create namespace dbc-gateway
kubectl -n dbc-gateway create secret generic dbc-gateway-google-credentials \
  --from-file=credentials.json=/secure/path/dbc-gateway-sa.json
```

The service account needs Cloud Logging write access in each configured `auditProject` and the Cloud Identity permissions required to look up groups and transitive memberships. The application uses Application Default Credentials through `GOOGLE_APPLICATION_CREDENTIALS`.

Create or issue the TLS Secret named `dbc-gateway-tls` in the same namespace. It must contain `tls.crt` and `tls.key` for `example.invalid`.

The Pod runs as the distroless `nonroot` user/group (`65532:65532`). Its pod-level `fsGroup` makes the projected Secret files readable through their `0440` group bit without making them world-readable.

## Build

```bash
docker build -f Containerfile --build-arg VERSION=dev -t REGISTRY/dbc-gateway:TAG .
docker push REGISTRY/dbc-gateway:TAG
```

Before deployment, update:

- `REGISTRY/dbc-gateway:TAG`
- the private/WireGuard MySQL address in the ConfigMap
- groups and audit projects
- the certificate Secret or cert-manager Certificate

Validate without changing the cluster:

```bash
kubectl apply --dry-run=client -f deploy/kubernetes/dbc-gateway.yaml
```

This template intentionally does not include NetworkPolicy because the current cluster networking does not enforce it. Enforce ingress and egress with Calico/Cilium or host firewall rules before production.

The JSON key is a transitional credential. WIF can later replace the Google Secret by changing the credential volume and keeping the same `GOOGLE_APPLICATION_CREDENTIALS` path.
