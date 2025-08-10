# Deployment Security

## GitHub Actions Security Measures

### Secret Protection

1. **Fork PR Protection**: Secrets are NOT available to workflows triggered by pull requests from forks
2. **First-time Contributor Approval**: New contributors require manual approval before workflows run
3. **Secret Masking**: GitHub automatically masks secrets in logs

### Our Additional Protections

1. **Environment Protection**: The deployment workflow uses the `trunk` environment which can be configured with:
   - Required reviewers
   - Deployment branches restrictions
   - Wait timer before deployment

2. **Workflow Conditions**: The deploy workflow only runs when:
   - CI passes on the main branch (not from PRs)
   - OR manually triggered by authorized users

3. **Branch Protection**: Ensure `main` branch has:
   - Required PR reviews
   - Required status checks (CI must pass)
   - No direct pushes

## Setting Up Environment Protection (Recommended)

1. Go to Settings → Environments in your GitHub repository
2. Create a "trunk" environment
3. Configure protection rules:
   - Add required reviewers
   - Restrict deployment branches to `main`
   - Add any required wait time

4. Move your `FLY_API_TOKEN` secret to the trunk environment:
   - Remove it from repository secrets
   - Add it to the trunk environment secrets

## Security Best Practices

1. **Rotate API tokens regularly**
2. **Use least-privilege tokens** - Create Fly.io tokens with only necessary permissions
3. **Monitor deployments** - Set up notifications for production deployments
4. **Audit workflow changes** - Review any PR that modifies `.github/workflows/`

## What Attackers Can't Do

Even if an attacker:
- Opens a PR with modified workflows → No access to secrets
- Tries to echo secrets → GitHub masks them
- Modifies the workflow file → Environment protection blocks unauthorized deployments
- Gets their PR merged → Branch protection requires reviews

## Emergency Response

If you suspect compromise:
1. Immediately revoke the Fly.io API token: `fly auth revoke <token>`
2. Generate a new token: `fly auth token`
3. Update the GitHub secret
4. Review deployment logs for unauthorized activity