<img src="./logo.png" height="130" align="right" alt="Dynatrace logo">

# Steadybit extension-dynatrace

A [Steadybit](https://www.steadybit.com/) extension for [Dynatrace](https://www.dynatrace.com/).

Learn about the capabilities of this extension in
our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_dynatrace).

## Configuration

| Environment Variable               | Helm value | Meaning                                                                                                                                                  | Required | Default |
|------------------------------------|------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_EXTENSION_API_BASE_URL` |            | The Dynatrace API Base Url, like `https://{your-environment-id}.live.dynatrace.com/api`                                                                  | yes      |         |
| `STEADYBIT_EXTENSION_API_TOKEN`    |            | The Dynatrace [API Token](https://docs.dynatrace.com/docs/dynatrace-api/basics/dynatrace-api-authentication#create-token), see the required scopes below | yes      |         |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

When installed as linux package this configuration is in`/etc/steadybit/extension-dynatrace`.

## Dynatrace Permissions

The extension requires the following scopes:
- `entities.read`
- `events.ingest`
- `settings.write` (if you want to use the "Create Maintenance Window" action)
- `problems.read` (if you want to use the "Check Problem" action)

## Installation

We recommend that you install the extension with
our [official Helm chart](https://github.com/steadybit/extension-dynatrace/tree/main/charts/steadybit-extension-dynatrace).

### Helm

```bash
helm repo add steadybit-extension-dynatrace https://steadybit.github.io/extension-dynatrace
helm repo update
```

```bash
helm upgrade steadybit-extension-dynatrace \
  --install \
  --wait \
  --timeout 5m0s \
  --create-namespace \
  --namespace steadybit-agent \
  steadybit-extension-dynatrace/steadybit-extension-dynatrace`
```

### Docker

You may alternatively start the Docker container manually.

```bash
docker run \
  --env STEADYBIT_LOG_LEVEL=info \
  --expose 8090 \
  ghcr.io/steadybit/extension-dynatrace:latest
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more
information.

### Linux Package

Please use
our [agent-linux.sh script](https://docs.steadybit.com/install-and-configure/install-agent/install-on-linux-hosts)
to install the extension on your Linux machine.
The script will download the latest version of the extension and install it using the package manager.

After installing configure the extension by editing `/etc/steadybit/extension-dynatrace` and then restart the service.

## Proxy

To communicate to Dynatrace via a proxy, we need the environment variable `https_proxy` to be set.
This can be set via helm using the extraEnv variable

```bash
--set "extraEnv[0].name=HTTPS_PROXY" \
--set "extraEnv[0].value=https:\\user:pwd@CompanyProxy.com:8888"
```
