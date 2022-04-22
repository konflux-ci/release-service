# Milestone 4 demo

This directory contains the resources needed for running the release demo for Milestone 4.

## Environments

This demo can run in two different environments:

* dev: This environment uses an empty CRC cluster with Openshift Pipelines preinstalled.
* staging: Meant to be used in a cluster bootstrapped with infra-deployments.

## Running the demo

To run the demo execute the following commands:

```bash
$ git clone https://github.com/redhat-appstudio/release-service.git
$ cd release-service/demo/m4
$ kubectl apply -k overlays/<environment>
```

## What to expect

After running the commands above, you should expect to find:

* a Build PipelineRun for the component `m4-component`;
* a Release created in the `demo` namespace;
* a Release PipelineRun created in the `managed` namespace.

The following command can be used to inspect the result of the release:
```bash
oc get release -n demo -o wide
NAME            COMPONENT      TRIGGER     SUCCEEDED   REASON      PIPELINERUN         START TIME   COMPLETION TIME   AGE
release-crndf   m4-component   automated   True        Succeeded   m4-strategy-stk9q   41s          32s               50s
```
