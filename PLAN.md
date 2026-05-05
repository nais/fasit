# UI Feedback Backlog

Source: user feedback (translated from Norwegian), 2026-05-05.

This is a **backlog**, not an execution plan. Items are independent UX
improvements. When picking one up, write a focused plan for it (and run it past
Momus if non-trivial).

## Visual Polish

- [ ] Improve contrast across the UI, especially in lists like Features and
      Deployments.
- [~] Unify visual style (contrast/colors) between Rollouts and Deployments
      pages — they currently differ widely. Quick wins done: relative-time
      timestamps with hover-for-absolute on rollouts list+detail, mirroring
      deployments. Status helpers verified equivalent. Structural alignment
      (table layout) intentionally not pursued — different data shapes.
- [x] Add more breathing room in headings and lists; increase spacing between
      logo and text (header is too compressed).
- [x] Larger font sizes (base font bumped 14px → 16px). Better use of available
      space still pending — likely improves naturally as more functionality
      from the original UI is ported.

## Status Indicators (failed reconciles/deployments)

- [ ] Front page: indicator for environments with failed reconciles/deployments.
- [x] Deployments page: failure indicator next to the feature name. (Pill
      badge "✗ N failed" / "⏳ N pending" on the feature group summary row.)
- [x] Features page: failure indicator next to feature name in sidebar
      (✗N / ⏳N pill, computed via deployment status aggregation).

## Feature / Environment / Overview

- [ ] Make secret configs editable.
- [x] Show which chart version is deployed to the environment, with an
      indicator if the deployed version is lagging behind the latest.
      Done on Feature/Status (versionCell shows mismatch: "1.0.0 → 2.0.0")
      and Feature/Environment/Overview ("Current version" vs "Chart version").
- [x] Show when the chart was last deployed to this environment.
      Done on Feature/Status ("Last deployed" column) and
      Feature/Environment/Logs subtitle.
- [x] Add a clear way to trigger a Redeploy.

## Feature / Status

- [ ] Per-environment enable/disable toggle directly on Feature/Status, so the
      user does not have to drill into each environment to disable.
- [x] Show "Last modified" as relative time (done — cells render "5m ago" etc.
      with absolute timestamp on hover). Renamed column to "Last update" on
      Feature/Status table and Environment/Feature/Logs subtitle to honestly
      reflect that it tracks any deploy-instruction activity (created or
      status-updated), not user-initiated changes.
- [x] Add a "Last deployed" column. Sourced from latest deploy_instruction
      with status='deployed' (new query DeployInstructionsLatestDeployedForFeature).
      Renders "never" when the feature has never successfully deployed in
      that environment. Added to Feature/Status table and Logs subtitle.
- [x] Move the "Tenant" column to the left of "Environment". Reading order
      tenant → environment is more natural than the reverse.

## Original feedback (Norwegian, verbatim)

> For dårlige kontraster, spesielt i lister som feauters og deployments.
> Er det en grunn til at utseende på rollouts og deployments er vidt forskjellig
> kontrast og fargemessig?
> for mye komprimering (for lite luft) i headinger og lister (og mellom logo og
> tekst.
> Dårlig utnyttelse av plass (det kommer sikkert når man får fylt på med flere
> funksjoner fra det originale), større font.
> Forsiden: Indikator for miljøer som har feilede reconciles/deployments.
> Tilsvarende kunne ligget på Deployments-siden ved siden av featurenavnet,
> evt. på Features-siden.
> Feature/Environment/Overview: Secret configs er ikke redigerbare.
> Feature/Environment/Overview: Hvilken versjon av chartet er deployet til
> miljøet (evt. med en indikator som sier ifra om versjonen henger etter). Når
> ble den sist deployet.
> Feature/Environment/Overview: Hvordan trigger jeg en "Redeploy"?
> Feature/Status: Enkel toggle for enable/disable per miljø. Hvis jeg skal
> disable noe så må jeg nå inn på hvert enkelt miljø.
> Feature/Status: "Last modified" kunne gjerne vært i relativ tid. Denne virker
> også å ha en annen betydning enn "Last modified" i Feature/Environment/Logs.
> Feature/Status: Litt avhengig av punktet ovenfor: kan jeg ønske meg en
> "Last deployed"-kolonne?
> Feature/Status: "Tenant"-kolonnen burde ligge til venstre for "Environment".
> Jeg synes det er lettere å finne tenant -> miljø kontra omvendt.
