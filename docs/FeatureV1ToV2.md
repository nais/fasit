# Converting a V1 feature to V2

## Convert yaml

Start by converting the old feature v2 to the new format:

```bash
go run ./cmd/convert [featurename]
```

Check the output for errors and things to check. Note them down or modify the v1 feature file directly.

The beginning of the output is recommendations for updates to your `Chart.yaml`. The second part is what will become your `Feature.yaml`.

You can also generate only the `Feature.yaml` by running (This will suppress Checks and Warnings):

```bash
go run ./cmd/convert --feature-only [featurename]
```

**If the chart is part of the `helm-charts` repo, move the chart from `charts/` to `features/`**

## Update your chart

Update your `Chart.yaml` with the recommendations from the conversion tool.

Create a new `Feature.yaml` file in the same directory as your `Chart.yaml` and copy the output from the conversion tool into it.

**Note:** If the chart exists in the helm-charts repository, move the chart from `charts/` to `features/`.

## Create rollout workflow

**If the chart is part of the `helm-charts` repo, you can skip this step as long as the feature exists in the `features/` directory.**

Create a new workflow file in the `.github/workflows/` directory. An example workflow file is:

```yaml
name: Build and release feature

on:
  push:
    branches:
      - main
    paths:
      - "features/**"
env:
  CHART_PATH: "features/chartname" # Update with chart name
  HELM_VERSION: "3.10.2"
  GOOGLE_REGISTRY: "europe-north1-docker.pkg.dev"

jobs:
  build_push:
    name: Build and push

    runs-on: ubuntu-latest
    permissions:
      contents: "read"
      actions: "read"
      id-token: "write"
    steps:
      - uses: actions/checkout@v3

      - uses: azure/setup-helm@v3
        name: "Setup Helm"
        with:
          version: ${{ env.HELM_VERSION }}

      - name: Build Chart
        id: build_chart
        run: |-
          suffix="$(date +%Y%m%d%H%M%S)"
          orig_version=$(yq '.version' < "${{ env.CHART_PATH}}/Chart.yaml")
          new_version="${orig_version}-$suffix"
          sed -i "s/^version: .*/version: $new_version/g" "${{ env.CHART_PATH}}/Chart.yaml"

          helm dependency update "${{ env.CHART_PATH}}"
          helm package "${{ env.CHART_PATH}}" --destination .

          name=$(yq '.name' < "${{ env.CHART_PATH}}/Chart.yaml")
          echo "name=$name" >> $GITHUB_OUTPUT
          echo "chart=$name-$new_version.tgz" > $GITHUB_OUTPUT
          echo "version=$new_version" >> $GITHUB_OUTPUT

      - id: "auth"
        name: "Authenticate to Google Cloud"
        uses: "google-github-actions/auth@v1.1.0"
        with:
          workload_identity_provider: ${{ secrets.NAIS_IO_WORKLOAD_IDENTITY_PROVIDER }}
          service_account: "{{UPDATE}}@nais-io.iam.gserviceaccount.com" # Update with service account name from nais-io-terraform-modules
          token_format: "access_token"

      - name: "Log in to Google Artifact Registry"
        run: |-
          echo '${{ steps.auth.outputs.access_token }}' | docker login -u oauth2accesstoken --password-stdin https://${{ env.GOOGLE_REGISTRY }}

      - name: Push Chart
        run: |-
          chart="${{ steps.build_chart.outputs.chart }}"
          echo "Pushing: $chart"
          helm push "$chart" oci://${{ env.GOOGLE_REGISTRY }}/nais-io/nais/feature
    outputs:
      name: ${{ steps.build_chart.outputs.name }}
      version: ${{ steps.build_chart.outputs.version }}

  rollout:
    needs:
      - build_push
    runs-on: fasit-deploy
    permissions:
      id-token: write
    steps:
      - uses: nais/fasit-deploy@v2
        with:
          chart: oci://${{ env.GOOGLE_REGISTRY }}/nais-io/nais/feature/${{ needs.build_push.outputs.name }}
          version: ${{ needs.build_push.outputs.version }}
```

## Your first rollout

When you have pushed your changes to the repository, and the action has completed, a rollout has been created in Fasit.

If it's a new feature you must enable it in at least one CI environment (Teannt dev-nais, ci and/or management) for the rollout to start.

If it succeeds in all possible environments it will be promoted to a feature in Fasit, which will be deployed to all environments.
