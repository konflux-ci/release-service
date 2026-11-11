# Release Service integration pipeline

`konflux-e2e-tests-pipeline.yaml` provisions a Kind cluster on IBM Cloud,
hardens it, deploys Konflux with the release-service image under test, runs
the existing E2E suite, and destroys the cluster in `finally`.

## Infrastructure

The pipeline uses integration catalog commit
`2d0243d6b0ad1e6f48a2eadd7c5cc41c39b8b52f` for IBM provision Task v0.4,
deprovision Task v0.1, and their nested StepActions. This includes
protect-control-plane v0.4 with helper log retries and artifact collection,
and mapt v0.14.6.

The instance profile is `cxf-32x64` (32 vCPUs, 64 GiB), with Kubernetes
`v1.37` and hardening enabled. The Task's tuning defaults match that profile.
A short ID derived from the PipelineRun UID is shared by provision and
deprovision, avoiding IBM's resource-name length limit.

The CI namespace must contain the Secret selected by
`ibmcloud-credentials-secret` (default `mapt-kind-secret-ibm`), with these keys:

- `IBMCLOUD_API_KEY`
- `IBMCLOUD_COS_ACCESS_KEY_ID`
- `IBMCLOUD_COS_SECRET_ACCESS_KEY`
- Optional: `IBMCLOUD_COS_ENDPOINT`

`ibmcloud-backend-url` defaults to `s3://konflux-mapt-state/mapt/kind`.
The COS bucket must already exist and be accessible to those credentials.
The pipeline appends the generated cluster ID to this prefix and passes the
same state URL to both Tasks. Override the prefix for another COS bucket.

The existing `konflux-test-infra-secret` parameter selects the registry
credentials used for the provision and cleanup artifacts.

## Diagnostics

The provision Task uploads `kind-ibm-provision-<TaskRun UID>.log` and any
successfully captured helper logs (`cp-patch-*.log`, `setup-node-*.log`) to
the PipelineRun's existing OCI artifact, after secret scanning. Failed runs
also attempt cluster-resource collection during deprovision.

The PR analyzer still processes JUnit and E2E logs. It currently requires an
exact provisioning-log filename and cannot select the IBM Task's UID-based
filename. Automatic provisioning-log analysis needs a separate analyzer
update; the IBM logs remain available in the OCI artifact unless secret
scanning removes them.
