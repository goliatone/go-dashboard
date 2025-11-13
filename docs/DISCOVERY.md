# Widget & Provider Discovery

Phase 9 introduces manifest-driven discovery so teams can register third-party
widgets without touching Go code. Manifests describe widget definitions,
provider metadata, and ownership details, while the new `widgetctl` CLI scaffolds
stubs and keeps manifests consistent.

![Manifest workflow](images/analytics-demo.svg)

## Manifest Format

Manifests are JSON or YAML documents with the following shape:

```yaml
version: 1
name: community-pack
package: github.com/example/go-dashboard-community
widgets:
  - definition:
      code: community.widget.pipeline_health
      name: Pipeline Health
      description: Tracks CI/CD duration and failure rates.
      category: ops
      schema:
        type: object
        properties:
          repos:
            type: array
            items:
              type: string
    provider:
      name: Pipeline Health Provider
      summary: Calls the CI insights API and emits refresh events.
      entry: github.com/example/go-dashboard-community.NewPipelineHealthProvider
      package: github.com/example/go-dashboard-community
      docs_url: https://example.com/dashboard/community/pipeline
      capabilities: ["html", "json", "refresh"]
      channel: community
    maintainers:
      - oss@example.com
    tags: ["ci", "cd", "ops"]
```

- `version` &mdash; format version (currently `1`).
- `package`, `homepage` &mdash; optional metadata shown in docs/storefronts.
- `definition` &mdash; mirrors `dashboard.WidgetDefinition`.
- `provider` &mdash; discovery metadata (factory entry point, docs link, capabilities).
- `maintainers`/`tags` &mdash; optional curation metadata.

Reference manifests live in `docs/manifests/` (`community.widgets.yaml`,
`internal.widgets.json`). Use them as fixtures when authoring your own packs.

### Localized Strings

Manifests may provide localized metadata alongside the default strings:

```yaml
definition:
  code: community.widget.pipeline_health
  name: Pipeline Health
  name_localized:
    es: Salud del pipeline
  description: Tracks CI/CD durations and failure counts across repositories.
  description_localized:
    es: Registra la duración y fallos de CI/CD por repositorio.
```

`dashboard.ResolveLocalizedValue(localizedMap, locale, fallback)` resolves the
best translation at runtime (e.g., `es-MX` falls back to `es` before the default
string). Providers and templates can combine this helper with the new
`TranslationService` to keep manifests, runtime data, and presentation in sync.

## Loading Manifests

The registry now parses manifests directly:

```go
reg := dashboard.NewRegistry()
if _, err := reg.LoadManifestFile("docs/manifests/community.widgets.yaml"); err != nil {
    log.Fatal(err)
}

service := dashboard.NewService(dashboard.Options{
    Registry: reg,
    WidgetStore: myStore,
})
```

`LoadManifestFile` validates the document (version + duplicates) and records
provider metadata so queries/transports can expose discovery information. Use
`reg.ProviderMetadata("community.widget.pipeline_health")` if you need to surface
docs links or capabilities in your UI.

## Scaffolding via CLI

`cmd/widgetctl` and the corresponding Taskfile target keep manifests and provider
stubs in sync:

```bash
./taskfile dashboard:widgets:scaffold \
  --code community.widget.pipeline_health \
  --name "Pipeline Health" \
  --description "Tracks CI/CD durations and failure counts." \
  --manifest docs/manifests/community.widgets.yaml \
  --docs-url https://example.com/dashboard/community/pipeline
```

The command will:

1. Create/update the manifest entry (sorted by `definition.code`), ensuring the
   schema placeholder compiles.
2. Generate a provider stub inside `components/dashboard/providers/` unless
   `--skip-provider` is supplied.

Use `--schema-path` to point at a JSON Schema file, `--capabilities` to record
supported transports (HTML, JSON, SSE), and `--overwrite` to replace an existing
entry or stub.

## Sharing & Validation

- Commit manifests alongside code so reviewers can spot changes.
- Keep one pack per file and bump the `version` field only when the format
  changes (the loader rejects unsupported versions).
- Run `./taskfile dashboard:manifests:lint` (see below) in CI to catch schema
  errors or duplicate widget codes before merging.
- When publishing a pack for others, include the manifest plus any templates,
  provider packages, and docs referenced inside the file so integrators can
  wire them up quickly.
