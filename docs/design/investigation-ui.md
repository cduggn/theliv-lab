# Investigation UI bridge

This change presents existing Theliv diagnostic results in an investigation
workspace. Recommendations still come from the existing Go investigators; no
LLM, agent runtime, or automated cluster changes are introduced.

## Review

- Select a cluster and namespace. Search findings or filter by type/signal.
- Review Overview, Evidence, and Resources; expand recommendations and commands.
- Copy a finding link and reload it. The link reloads current results; it is not
  a historical investigation snapshot. Legacy issue links remain supported.
- Check no-findings, filtered-empty, unavailable-events, and failed-refresh states.
- Review keyboard focus, light/dark themes, and narrow-screen layout.

## Transfer into a workplace checkout

The personal repository is `cduggn/theliv-lab`. Its `ui-refresh` branch contains
one UI commit on upstream `46091262b1c85553bb20dcaabc69fe37643d4f75`.
Its `main` branch adds a separate optional local k3s setup commit.

From a clean workplace checkout, run:

```sh
git fetch https://github.com/cduggn/theliv-lab.git ui-refresh
git switch -c review/theliv-investigations
git cherry-pick FETCH_HEAD
```

Resolve any differences against the workplace version before testing. This
workflow creates no pull request and pushes nothing to fidelity/theliv.

## Validation

Use the project's Angular 12 toolchain (the local demo uses Node 14.21.3 and
npm 8.19.4 in a container). Dependencies are pinned in package-lock.json.

```sh
cd website
npm ci --legacy-peer-deps --no-audit --no-fund
npm run test:investigations
./node_modules/.bin/tsc --project tsconfig.spec.json --noEmit
./node_modules/.bin/ng build --configuration development --base-href /theliv/ --deploy-url /theliv/
```

The existing API prefix remains `/theliv-api/v1`. Keep your workplace's normal
authentication, API routing, and deployment configuration. For this iteration,
19 focused tests, TypeScript checks, and the development build passed. Live
browser verification was unavailable in the preparing session.

## Files

- `website/package-lock.json`
- `website/package.json`
- `website/scripts/test-investigations.cjs`
- `website/src/app/app-routing.module.ts`
- `website/src/app/app.component.scss`
- `website/src/app/components/navigation-bar/navigation-bar.component.html`
- `website/src/app/components/navigation-bar/navigation-bar.component.scss`
- `website/src/app/kube-platform/investigation.spec.ts`
- `website/src/app/kube-platform/investigation.ts`
- `website/src/app/kube-platform/kube-platform.component.html`
- `website/src/app/kube-platform/kube-platform.component.scss`
- `website/src/app/kube-platform/kube-platform.component.spec.ts`
- `website/src/app/kube-platform/kube-platform.component.ts`
- `website/src/app/services/kubernetes.service.spec.ts`
- `website/src/app/services/kubernetes.service.ts`
- `website/src/app/shared/util/inline-http-errors.ts`
- `website/src/app/shared/util/unauthorized-interceptor.ts`
- `website/src/styles.scss`
