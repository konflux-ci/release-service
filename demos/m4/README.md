# Milestone 4 demo

This directory contains the resources needed for running the release demo for Milestone 4.

## Environments

This demo can run in two different environments:

* dev: This environment uses an empty CRC cluster with Openshift Pipelines preinstalled.
* staging: Meant to be used in a cluster bootstrapped with infra-deployments.

## Running the happy path

To run the demo's happy path execute the following commands:

```bash
$ git clone -b 0.1.0 --depth 1 https://github.com/redhat-appstudio/release-service.git
$ cd release-service/demo/m4
$ kubectl apply -k overlays/<environment>
```

### What to expect

After running the commands above, you should expect to find:

* a Build PipelineRun for the component `m4-component`;
* a Release created in the `demo` namespace;
* a Release PipelineRun created in the `managed` namespace.

The following command can be used to inspect the result of the release:
```bash
oc get release -n demo -o wide
NAME            COMPONENT      SUCCEEDED   REASON      PIPELINERUN         START TIME   COMPLETION TIME   AGE
release-crndf   m4-component   True        Succeeded   m4-strategy-stk9q   41s          32s               50s
```

The important part to notice here is that the Release succeeded and that we keep a reference to the Release PipelineRun.

## Running failure scenarios

There are various checks that must pass in the release process.
The `failures/` directory has scenario files for showing ways in which a Release can fail.

### Missing matching ReleaseLink in managed workspace

A Release cannot be executed unless there is a ReleaseLink in the managed workspace which matches the one in the user workspace.
Two ReleaseLinks are considered matching if they both specify the same application and each of them target the other's workspace.
This scenario shows the situation where a Release is created in the user workspace with a ReleaseLink to a managed workspace, but there is no matching ReleaseLink in the managed workspace.

```bash
$ git clone -b 0.1.0 --depth 1 https://github.com/redhat-appstudio/release-service.git
$ cd release-service/demo/m4/failures
$ oc apply -f missing_matching_release_link.yaml
```

#### What to expect

After running the commands above, you should expect to find:

* a ReleaseLink `test-releaselink` in the `matching-scenario-user` namespace;
* a Release `test-release` created in the `matching-scenario-user` namespace;
* the `REASON` field of the `test-release` Release set to `Error`.

The following command can be used to inspect the release:
```bash
oc get release -n matching-scenario-user -o wide
NAME            COMPONENT      SUCCEEDED   REASON      PIPELINERUN         START TIME   COMPLETION TIME   AGE
test-release    test           False       Error                                                          29s
```
```bash
oc get -n matching-scenario-user release test-release -o yaml
---------------------------<snip>--------------------------
    message: no ReleaseLink found in target workspace 'matching-scenario-managed'
      with target 'matching-scenario-user' and application 'test'
---------------------------<snip>--------------------------
```

The controller fails to find the matching ReleaseLink and thus marks the Release with Error and never starts a Release PipelineRun for it.

To delete the resources used in this scenario, run the following command:
```bash
oc delete -f missing_matching_release_link.yaml
```

### ReleaseLink targeting itself

A ReleaseLink resource is considered invalid if its target references its own workspace.
For the sake of milestone 4, a namespace is treated as a workspace.
This failure scenario shows what happens when a user creates a ReleaseLink resource that targets its own namespace.

```bash
$ git clone -b 0.1.0 --depth 1 https://github.com/redhat-appstudio/release-service.git
$ cd release-service/demo/m4/failures
$ oc apply -f release_link_targets_own_workspace.yaml
```

#### What to expect

After running the commands above, you should expect to find:

* the ReleaseLink resource `targeting-itself` is not created due to failing webhook validation.

### Missing ReleaseStrategy

A Release PipelineRun must execute a ReleaseStrategy that is stored in the managed workspace.
The ReleaseStrategy is defined in the ReleaseLink resource in the managed workspace.
The Release controller checks for the ReleaseStrategy's existence and will only start a Release PipelineRun if it finds the ReleaseStrategy.
This scenario is for showing what happens if the ReleaseStrategy does not exist.

```bash
$ git clone -b 0.1.0 --depth 1 https://github.com/redhat-appstudio/release-service.git
$ cd release-service/demo/m4/failures
$ oc apply -f missing_release_strategy.yaml
```

#### What to expect

After running the commands above, you should expect to find:

* a ReleaseLink `test-releaselink-user` in the `releasestrategy-scenario-user` namespace;
* a ReleaseLink `test-releaselink-managed` in the `releasestrategy-scenario-managed` namespace;
* a Release `test-release` created in the `releasestrategy-scenario-user` namespace;
* the `REASON` field of the `test-release` Release set to `Error`.

The following command can be used to inspect the release:
```bash
oc get release -n releasestrategy-scenario-user -o wide
NAME            COMPONENT      SUCCEEDED   REASON      PIPELINERUN         START TIME   COMPLETION TIME   AGE
test-release    test           False       Error                                                          29s
```
```bash
oc get -n releasestrategy-scenario-user release test-release -o yaml
---------------------------<snip>--------------------------
    message: ReleaseStrategy.appstudio.redhat.com "missing-releasestrategy" not found
    reason: Error
---------------------------<snip>--------------------------
```

The controller fails to find the `missing-releasestrategy` ReleaseStrategy in the `releasestrategy-scenario-managed` workspace.
Because of this, it marks the Release as `Error` and never starts a Release PipelineRun for it.

To delete the resources used in this scenario, run the following command:
```bash
oc delete -f missing_release_strategy.yaml
```

### Release Pipeline not found

If all the ReleaseLinks are valid and the referenced ReleaseStrategy exists, the last check is that the ReleaseStrategy refers to a valid pipeline.
This scenario shows a Release resource that fails because the ReleaseStrategy in the ReleaseLink refers to a non-existing pipeline.

```bash
$ git clone -b 0.1.0 --depth 1 https://github.com/redhat-appstudio/release-service.git
$ cd release-service/demo/m4/failures
$ oc apply -f missing_release_pipeline.yaml
```

#### What to expect

After running the commands above, you should expect to find:

* a ReleaseLink `test-releaselink-user` in the `releasepipeline-scenario-user` namespace;
* a ReleaseLink `test-releaselink-managed` in the `releasepipeline-scenario-managed` namespace;
* a ReleaseStrategy `test-strategy` in the `releasepipeline-scenario-managed` namespace;
* a Release `test-release` created in the `releasepipeline-scenario-user` namespace;
* the `REASON` field of the `test-release` Release set to `Failed`.

The following command can be used to inspect the release:
```bash
oc get release -n releasepipeline-scenario-user -o wide
NAME           COMPONENT   SUCCEEDED   REASON   PIPELINERUN           START TIME   COMPLETION TIME   AGE
test-release   test        False       Failed   test-strategy-d2t22   38s          38s               38s
```
```bash
oc get -n releasepipeline-scenario-user release test-release -o yaml
---------------------------<snip>--------------------------
    message: 'Error retrieving pipeline for pipelinerun releasepipeline-scenario-managed/test-strategy-d2t22:
      error when listing pipelines for pipelineRun test-strategy-d2t22: could not
      find object in image with kind: pipeline and name: missing-release-pipeline'
    reason: Failed
---------------------------<snip>--------------------------
```

The controller searches for the `missing-release-pipeline` in the `quay.io/hacbs-release/m4:0.1-alpine` bundle.
That pipeline does not exist, so the `Release` is marked with `Failed`.

To delete the resources used in this scenario, run the following command:
```bash
oc delete -f missing_release_pipeline.yaml
```
