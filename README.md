<img src="./logo.png" height="130" align="right" alt="Dynatrace logo">

# Steadybit extension-dynatrace

A [Steadybit](https://www.steadybit.com/) extension for [Dynatrace](https://www.dynatrace.com/).

Learn about the capabilities of this extension in
our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_dynatrace).

## Configuration

| Environment Variable               | Helm value | Meaning                                                                                                                                                  | Required | Default |
|------------------------------------|------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_EXTENSION_API_BASE_URL` |            | The Dynatrace API Base Url, like `https://{your-environment-id}.live.dynatrace.com/api`                                                                  | yes      |         |
| `STEADYBIT_EXTENSION_UI_BASE_URL`  |            | The Dynatrace UI Base Url, like `https://{your-environment-id}.apps.dynatrace.com/ui`                                                                    | yes      |         |
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

### Kubernetes

Detailed information about agent and extension installation in kubernetes can also be found in
our [documentation](https://docs.steadybit.com/install-and-configure/install-agent/install-on-kubernetes).

#### Recommended (via agent helm chart)

All extensions provide a helm chart that is also integrated in the
[helm-chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-agent) of the agent.

You must provide additional values to activate this extension.

```
--set extension-dynatrace.enabled=true \
--set extension-dynatrace.dynatrace.apiBaseUrl={{YOUR_API_BASE_URL}} \
--set extension-dynatrace.dynatrace.uiBaseUrl={{YOUR_UI_BASE_URL}} \
--set extension-dynatrace.dynatrace.apiToken={{YOUR_API_TOKEN}} \
```

Additional configuration options can be found in
the [helm-chart](https://github.com/steadybit/extension-dynatrace/blob/main/charts/steadybit-extension-dynatrace/values.yaml) of the
extension.

#### Alternative (via own helm chart)

If you need more control, you can install the extension via its
dedicated [helm-chart](https://github.com/steadybit/extension-dynatrace/blob/main/charts/steadybit-extension-dynatrace).

```bash
helm repo add steadybit-extension-dynatrace https://steadybit.github.io/extension-dynatrace
helm repo update
helm upgrade steadybit-extension-dynatrace \
  --install \
  --wait \
  --timeout 5m0s \
  --create-namespace \
  --namespace steadybit-agent \
  --set dynatrace.apiBaseUrl={{YOUR_API_BASE_URL}} \
  --set dynatrace.uiBaseUrl={{YOUR_UI_BASE_URL}} \
  --set dynatrace.apiToken={{YOUR_API_TOKEN}} \
  steadybit-extension-dynatrace/steadybit-extension-dynatrace`
```

### Linux Package

Please use
our [agent-linux.sh script](https://docs.steadybit.com/install-and-configure/install-agent/install-on-linux-hosts)
to install the extension on your Linux machine. The script will download the latest version of the extension and install
it using the package manager.

After installing, configure the extension by editing `/etc/steadybit/extension-dynatrace` and then restart the service.

## Extension registration

Make sure that the extension is registered with the agent. In most cases this is done automatically. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-discovery) for more
information about extension registration and how to verify.

## Proxy

To communicate to Dynatrace via a proxy, we need the environment variable `https_proxy` to be set.
This can be set via helm using the extraEnv variable

```bash
--set "extraEnv[0].name=HTTPS_PROXY" \
--set "extraEnv[0].value=https:\\user:pwd@CompanyProxy.com:8888"
```
