# Upgrading

## Normal OLM Upgrade

Run a new semver tag:

```bash
./build-and-deploy.sh v1.0.2 --olm
```

The generated bundle uses the current installed CSV as `replaces` when present.

## Recover From A Bad Pending CSV

Inspect requirements:

```bash
oc get csv <csv-name> -n app-dashboard-operator -o yaml
```

Delete only the bad CSV when it owns failed install components:

```bash
oc delete csv <bad-csv-name> -n app-dashboard-operator
```

Then watch the replacement:

```bash
oc get csv,installplan,subscription,pods -n app-dashboard-operator
```

## Do Not Delete Running Plugin First

Keep the `app-dashboard` namespace running while fixing OLM unless the plugin
workload itself is the problem. The OLM-installed operator can reconcile the
existing plugin deployment after its CSV succeeds.
