# HACBS Release service

Release service is a Kubernetes operator to control the life cycle of HACBS-managed releases in the context of AppStudio.

## Running, building and testing the operator

This operator provides a [Makefile](Makefile) to run all the usual development tasks. This file can be used by cloning
the repository and running `make` over any of the provided targets.

### Running the operator locally

When testing locally (eg. a CRC cluster), the command `make run install` can be used to deploy and run the operator. 
If any change has been done in the code, `make manifests generate` should be executed before to generate the new resources
and build the operator.

### Build and push a new image

To build the operator and push a new image to the registry, the following commands can be used: 

```shell
$ make docker-build
$ make docker-push
```

These commands will use the default image and tag. To modify them, new values for `TAG` and `IMG` environment variables
can be passed. For example, to override the tag:

```shell
$ TAG=my-tag make docker-build
$ TAG=my-tag make docker-push
```

Or, in the case the image should be pushed to a different repository:

```shell
$ IMG=quay.io/user/release:my-tag make docker-build
$ IMG=quay.io/user/release:my-tag make docker-push
```

### Running tests

To test the code, simply run `make test`. This command will fetch all the required dependencies and test the code. The
test coverage will be reported at the end, once all the tests have been executed.

## Disabling Webhooks for local development

Webhooks require self-signed certificates to validate the resources. To disable webhooks during local development and
testing, export the `ENABLE_WEBHOOKS` variable setting its value to `false` or prepend it while running the operator
using the following command:

```shell
$ ENABLE_WEBHOOKS=false make run install
```
