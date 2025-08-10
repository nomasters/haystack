# Haystack Fly.io Deployment

This is a reference implementation for deploying Haystack to [Fly.io](https://fly.io), a platform that provides global app deployment with excellent support for UDP services.

## Overview

This deployment uses Haystack's hermetic build system to ensure:
- Only committed code is deployed
- Deployments are idempotent (same tree hash = no redeploy)
- Images are pulled from Docker Hub, not built on Fly.io
- Automatic tracking of deployed versions

**Storage Configuration:**
- Uses mmap storage backend for persistence (configured via `HAYSTACK_STORAGE` environment variable)
- 10GB persistent volume mounted at `/data` (configured via `HAYSTACK_DATA_DIR` environment variable)
- Data survives container restarts and deployments
- Efficient memory-mapped file access

## Prerequisites

1. **Fly.io Account and CLI**
   ```bash
   # Install flyctl
   brew install flyctl  # macOS
   # or
   curl -L https://fly.io/install.sh | sh  # Linux
   
   # Authenticate
   fly auth login
   ```

2. **Docker Hub**
   - Haystack images must be pushed to Docker Hub
   - Default repository: `nomasters/haystack`
   - Or configure your own with `DOCKER_REPO` environment variable

3. **Built and Pushed Images**
   ```bash
   # From the repository root
   cd ../../../  # Go to haystack root
   make docker-push
   ```

## Quick Start

1. **Deploy with defaults:**
   ```bash
   ./deploy.sh
   ```

   Note: On first deployment, Fly.io will automatically create a 10GB persistent volume for mmap storage.

2. **Custom configuration:**
   ```bash
   FLY_APP_NAME=my-haystack FLY_REGION=lax ./deploy.sh
   ```

## How It Works

The `deploy.sh` script:

1. **Calculates tree hash** from current HEAD commit
2. **Verifies Docker image** exists with that tree hash
3. **Checks current deployment** to see if already running same version
4. **Generates fly.toml** with the correct image tag
5. **Deploys only if needed** (new tree hash or first deployment)

## Configuration

### Environment Variables

- `FLY_APP_NAME`: Fly.io app name (default: `haystack-kv`)
- `FLY_REGION`: Primary region (default: `ord` - Chicago)
- `DOCKER_REPO`: Docker repository (default: `nomasters/haystack`)
- `DOCKER_REGISTRY`: Docker registry (default: `docker.io`)

### Regions

Common Fly.io regions:
- `ord` - Chicago
- `lax` - Los Angeles
- `sea` - Seattle
- `ewr` - New Jersey
- `lhr` - London
- `ams` - Amsterdam
- `nrt` - Tokyo
- `syd` - Sydney

## Testing Your Deployment

Once deployed, test your Haystack instance:

```bash
# Get your app's URL
fly status --app haystack-kv

# Test with haystack client
echo "hello world" | haystack client set --endpoint haystack-kv.fly.dev:1337

# Get the hash and retrieve
haystack client get <hash> --endpoint haystack-kv.fly.dev:1337
```

## UDP on Fly.io

Important notes about UDP on Fly.io:
- UDP services get a dedicated IPv4 address
- No load balancing for UDP (each instance has its own IP)
- UDP ports are exposed globally
- Firewall rules can be configured if needed

## Scaling

For production deployments, you may want multiple instances:

```bash
# Scale to multiple regions
fly scale count 3 --app haystack-kv

# Set specific regions
fly regions set ord lax lhr --app haystack-kv
```

## Monitoring

```bash
# View logs
fly logs --app haystack-kv

# Check status
fly status --app haystack-kv

# SSH into instance (for debugging)
fly ssh console --app haystack-kv

# Check volume usage
fly ssh console --app haystack-kv -C "df -h /data"

# View mmap storage files
fly ssh console --app haystack-kv -C "ls -la /data"
```

## Customization

### Using Your Own Docker Images

1. Fork the Haystack repository
2. Configure your Docker Hub repository:
   ```bash
   export DOCKER_REPO=yourusername/haystack
   ```
3. Build and push your images:
   ```bash
   make docker-push
   ```
4. Deploy:
   ```bash
   DOCKER_REPO=yourusername/haystack ./deploy.sh
   ```

### Modifying fly.toml

The `fly.toml` file can be customized for your needs:
- Adjust VM resources (`cpus`, `memory_mb`)
- Modify environment variables (`HAYSTACK_ADDR`, `HAYSTACK_STORAGE`, `HAYSTACK_DATA_DIR`)
- Configure persistent volumes
- Set up multiple regions

## Troubleshooting

### Image Not Found

If you get "Docker image not found":
```bash
# Check if image exists locally
docker images | grep haystack

# Push to Docker Hub
cd ../../../  # Go to repo root
make docker-push
```

### App Already Exists

If the app name is taken:
```bash
FLY_APP_NAME=my-unique-haystack ./deploy.sh
```

### Connection Issues

UDP services on Fly.io:
- Make sure you're using port 1337
- Use the full hostname: `your-app.fly.dev:1337`
- Check firewall rules if applicable

## Clean Up

To remove your deployment:
```bash
fly apps destroy haystack-kv
```

## Security Considerations

- Haystack has no authentication by design
- Consider Fly.io's firewall features if you need access control
- Data is ephemeral (TTL-based expiration)
- No encryption in transit (add if needed for your use case)

## Support

- [Fly.io Documentation](https://fly.io/docs/)
- [Haystack Repository](https://github.com/nomasters/haystack)
- [Fly.io Community](https://community.fly.io/)

## License

This deployment example is provided under the same license as Haystack (The Unlicense).