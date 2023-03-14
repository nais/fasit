# fasit

## Features

Fasit contains a set of features that can be enabled for a tenant. Each feature has a definition in the `features` directory.

Documentation for each feature can be found in [`features/README.md`](./features/README.md).

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

## Verifying the fasit images and their contents

The images are signed "keylessly" (is that a word?) using [Sigstore cosign](https://github.com/sigstore/cosign).
To verify their authenticity run
```
cosign verify \
--certificate-identity "https://github.com/nais/fasit/.github/workflows/<filename>.yaml@refs/heads/main" \
--certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-<name>@sha256:<shasum>
```

The images are also attested with SBOMs in the [CycloneDX](https://cyclonedx.org/) format.
You can verify these by running
```
cosign verify-attestation --type cyclonedx  \
--certificate-identity "https://github.com/nais/fasit/.github/workflows/<filename>.yaml@refs/heads/main" \
--certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
europe-north1-docker.pkg.dev/nais-io/nais/images/fasit-<name>@sha256:<shasum>
```