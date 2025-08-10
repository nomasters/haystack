# Fly.io Deployment

This directory contains configuration for deploying Haystack to Fly.io.

## Automatic Deployment

The application is automatically deployed to Fly.io when changes are merged to the `main` branch via GitHub Actions.

### Setup Requirements

1. **Fly.io Account**: You need a Fly.io account with an app already created
2. **GitHub Secret**: Add your Fly.io API token as a GitHub secret named `FLY_API_TOKEN`

### Getting Your Fly.io API Token

```bash
fly auth token
```

### Manual Deployment

To deploy manually from your local machine:

```bash
cd examples/deployments/fly.io
fly deploy
```

### Monitoring

Check deployment status:
```bash
fly status
fly logs
```

### Configuration

The deployment configuration is in:
- `fly.toml` - Fly.io app configuration
- `Dockerfile` - Container definition
- `.github/workflows/deploy-fly.yaml` - GitHub Actions workflow

### Environment Variables

The following environment variables are configured in `fly.toml`:
- `HAYSTACK_ADDR`: Server binding address (fly-global-services:1337)
- `HAYSTACK_STORAGE`: Storage backend (memory)
- `HAYSTACK_LOG_LEVEL`: Logging level (debug/info/error/silent)

### UDP Service

The service runs on UDP port 1337 and is accessible via the Fly.io global anycast network.