# Milestone 5 demo

This directory contains the resources needed for running the release demo for Milestone 5.

# Bootstrapping

After bootstrapping the cluster with App Studio, you can prepare the demo by running the [bootstrap.sh](demos/m5/bootstrap.sh) script.
```
$ git clone https://github.com/redhat-appstudio/release-service.git
$ cd release-service/demos/m5
$ ./bootstrap.sh
```
This will create all the required resources to run the demo. Those are:
* Demo resources in the base directory.
* Quay secret to pull and push images (a password to the robot account will be requested).
* Cosign secret with the public key extracted from tekton-chains.

# Triggering the demo

To trigger the demo, the only thing needed after bootstrapping is to apply the [release.yaml](demos/m5/release.yaml) file, which will create a new Release in the cluster.
