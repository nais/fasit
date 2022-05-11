# fasit

## local dev setup

```
docker-compose up
make setup

# Run naisd
make local-naisd

# Run backend
make local

# Run frontend
cd frontend
npm run install
npm run dev
```

# Enroll tenant

These are some manual steps and configurations to do when enrolling a new tenant

## Management cluster

### feature: Cert-manager

| key                                                           | value                                                |
| ------------------------------------------------------------- | ---------------------------------------------------- |
| `global.leaderElection.namespace`                             | `nai-system`                                         |
| `installCRDs`                                                 | `true`                                               |
| `serviceAccount.annotations.iam\.gke\.io/gcp-service-account` | `acme-user@{ENV_PROJECT_ID}.iam.gserviceaccount.com` |
| `extraArgs`                                                   | `--issuer-ambient-credentials`                       |

### feature: management-certificates

| key                | value                          |
| ------------------ | ------------------------------ |
| `cloudDNS.project` | `ENV_PROJECT_ID`               |
| `cloudDNS.zone`    | `{PARTNER_NAME}-cloud-nais-io` |
| `partner`          | `{PARTNER_NAME}`               |
