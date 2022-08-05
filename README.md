# fasit

## local dev setup

```
docker-compose up

# Run backend
make local

# Build config
make setup

# Run naisd
make local-naisd

# Run frontend
cd frontend
echo "NEXT_PUBLIC_ENV=development" > .env.development.local
npm run install
npm run dev
```

# Enroll tenant

These are some manual steps and configurations to do when enrolling a new tenant

## Management cluster

### feature: cert-manager

| key                                                           | value                                                |
| ------------------------------------------------------------- | ---------------------------------------------------- |
| `global.leaderElection.namespace`                             | `nai-system`                                         |
| `installCRDs`                                                 | `true`                                               |
| `serviceAccount.annotations.iam\.gke\.io/gcp-service-account` | `acme-user@{ENV_PROJECT_ID}.iam.gserviceaccount.com` |
| `extraArgs`                                                   | `--issuer-ambient-credentials`                       |

### feature: management-certificates

| key                | value                          |
| ------------------ | ------------------------------ |
| `cloudDNS.project` | `{ENV_PROJECT_ID}`             |
| `cloudDNS.zone`    | `{PARTNER_NAME}-cloud-nais-io` |
| `partner`          | `{PARTNER_NAME}`               |

### feature: loadbalancer

Before enabling the loadbalancer feature, create a cloud armor policy in the GCP project of the cluster.

| key            | value                  |
| -------------- | ---------------------- |
| `certificates` | `wc-cloud-nais-io-tls` |
