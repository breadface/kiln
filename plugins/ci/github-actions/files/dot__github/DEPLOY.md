# Deploying {{APP_NAME}}

The **Deploy** workflow (`.github/workflows/deploy.yml`) builds every image,
pushes it to `{{REGISTRY_HOST}}/{{REGISTRY_NAMESPACE}}`, writes the
application's environment to `.env`, and runs `{{DEPLOY_COMMAND}}`. Start it
from the repository's **Actions** tab → **Deploy** → **Run workflow**.

## Repository secrets

Add these under **Settings → Secrets and variables → Actions**.

### Image registry

- `{{REGISTRY_PASSWORD_SECRET}}` — a token that can push and pull images on
  `{{REGISTRY_HOST}}` as `{{REGISTRY_USERNAME}}`.

### Deploy

<!-- kiln:insert deploy-secrets -->

### Application environment

Written to `.env` on every deploy.

<!-- kiln:insert env -->
