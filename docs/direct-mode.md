# Direct Mode Detection

The render-diff command supports two component detection modes:

- **appset** (default): Uses ArgoCD ApplicationSets to discover components and environments
- **direct**: Walks directory trees to find kustomization directories based on changed files

## When to use direct mode

Use `--detection-mode=direct` when your repository:

- Does not use ArgoCD ApplicationSets for deployment
- Has a simpler directory structure where environments are organized as subdirectories
- Uses direct kustomization paths without ApplicationSet generators

## Example repository structures

### Standard structure (appset mode)

```
repo/
  argo-cd-apps/
    overlays/
      development/
        app-of-apps.yaml          # ApplicationSet definitions
      staging/
        app-of-apps.yaml
  components/
    foo/
      base/
        kustomization.yaml
      overlays/
        development/
          kustomization.yaml
        staging/
          kustomization.yaml
```

### Simple structure (direct mode)

```
repo/
  components/
    foo/
      development/
        kustomization.yaml        # Found by walking from changed files
      staging/
        kustomization.yaml
      production/
        kustomization.yaml
```

## Usage

### Basic direct mode

```bash
./bin/render-diff --detection-mode=direct
```

This uses the default `--components-dir=components` and searches for kustomization directories under that path.

### Custom components directory

```bash
./bin/render-diff --detection-mode=direct --components-dir=services
```

### How direct mode works

1. Filters changed files to those under `--components-dir`
2. For each changed file, walks up the directory tree to find the nearest `kustomization.yaml` or `kustomization.yml`
3. Infers the environment from directory names in the path (development, staging, production)
4. Extracts optional cluster-specific directories from the path

### Environment inference

Direct mode recognizes these directory names as environments:

- `development` → Development
- `staging` → Staging
- `production` → Production

For example, a change to `components/foo/staging/config.yaml` will:

1. Walk up from `config.yaml` to find `components/foo/staging/kustomization.yaml`
2. Infer environment "Staging" from the directory name
3. Include this path in the staging environment diff

### Cluster-specific paths

Direct mode supports cluster-specific overlays:

```
components/
  foo/
    production/
      private/
        cluster-01/
          kustomization.yaml
        cluster-02/
          kustomization.yaml
```

The detector extracts the cluster directory (e.g., "cluster-01") and includes it in the component path for clearer diff output.

## Comparison with appset mode

| Feature | appset mode | direct mode |
|---------|-------------|-------------|
| Discovery method | Parse ApplicationSet generators | Walk directory tree from changed files |
| Repository structure | ArgoCD ApplicationSet-based | Simple kustomization hierarchy |
| Environment detection | From ApplicationSet targetRevision | From directory names in path |
| Performance | Reads all ApplicationSets upfront | Walks from changed files only |
| Use case | Complex multi-environment deployments | Simpler repository structures |

## Troubleshooting

### "No affected components detected"

This means no changed files were under `--components-dir`, or none of their parent directories contain a `kustomization.yaml`.

Check:

```bash
# Verify components directory exists
ls components/

# Verify kustomization files exist
find components/ -name 'kustomization.yaml'

# Check which files changed
git diff --name-only $(git merge-base HEAD main)
```

### Environment not recognized

If the detector cannot determine the environment for a path, it logs a warning and skips that component.

Ensure your directory structure includes one of the recognized environment names: `development`, `staging`, or `production`.

### Wrong components detected

Use debug logging to see the detection process:

```bash
./bin/render-diff --detection-mode=direct --log-file=/tmp/debug.log
cat /tmp/debug.log
```

This shows which kustomization directories were found and how environments were inferred.
