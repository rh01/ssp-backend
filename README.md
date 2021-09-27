# General idea
Build Status: [![Build Status](https://travis-ci.org/SchweizerischeBundesbahnen/ssp-backend.svg?branch=master)](https://travis-ci.org/SchweizerischeBundesbahnen/ssp-backend "Travis CI Jobs")

Docker Image: [![Docker Hub package][dockerhub-badge]][dockerhub-link]

[dockerhub-badge]: https://images.microbadger.com/badges/version/schweizerischebundesbahnen/ssp-backend.svg
[dockerhub-link]: https://hub.docker.com/r/schweizerischebundesbahnen/ssp-backend/tags?page=1&ordering=last_updated "Docker images"

We at [@SchweizerischeBundesbahnen](https://github.com/SchweizerischeBundesbahnen) own a lot of projects which receive changes all the time. As those settings are (and that is fine) limited to administrative roles, we had to do a lot of manual work like:

OpenShift:
- Creating new projects with certain attributes
- Updating project metadata like billing information
- Updating project quotas
- Creating service-accounts

Persistent storage:
- Creating gluster volumes
- Increasing gluster volume sizes
- Creating PV, PVC, Gluster Service & Endpoints in OpenShift

Billing:
- Creating billing reports for different platforms

AWS:
- Creating and managing AWS S3 Buckets

Sematext:
- Creating and managing sematext logsene apps

Because of that we built this tool which allows users to execute certain tasks in self service. The tool checks permissions & multiple defined conditions.

# Components
- The Self-Service-Portal Backend (as a container)
- The Self-Service-Portal Frontend (see https://github.com/SchweizerischeBundesbahnen/cloud-selfservice-portal-frontend)
- The GlusterFS-API server (as a sytemd service)

# Installation & Documentation
## Self-Service Portal
```bash
# Create a project & a service-account
oc new-project ose-selfservice-backend
oc create serviceaccount ose-selfservice

# Add a cluster policy for the portal:
oc create -f clusterRole-selfservice.yml

# Add policy to the service account
oc adm policy add-cluster-role-to-user ose:selfservice system:serviceaccount:ose-selfservice-backend:ose-selfservice
oc adm policy add-cluster-role-to-user admin system:serviceaccount:ose-selfservice-backend:ose-selfservice

# Use the token of the service account in the container
```

Just create a 'oc new-app' from the dockerfile.

### Config
[openshift/ssp-backend-template.json#L303](https://github.com/SchweizerischeBundesbahnen/ssp-backend/blob/master/openshift/ssp-backend-template.json#L303)

We are currently migrating from environment variables (with OpenShift template parameters) to a yaml config file. Most of the config options are compatible with both formats (can be set as environment variable or in the `config.yaml` file). The yaml config was introduced, because we needed more complex data structures.

e.g. The Openshift config must be set in `config.yaml` (see `config.yaml.example`):

```
openshift:
  - id: awsdev
    name: AWS Dev
    url: https://master.example.com:8443
    token: aeiaiesatehantehinartehinatenhiat
    glusterapi:
      url: http://glusterapi.com:2601
      secret: someverysecuresecret
      ips: 10.10.10.10, 10.10.10.11
  - id: awsprod
    name: AWS Prod
    url: https://master.example-prod.com
    token: aeiaiesatehantehinartehinatenhiat
    nfsapi:
      url: https://nfsapi.com
      secret: s3Cr3T
      proxy: http://nfsproxy.com:8000
```
To enable support for Ansible Tower jobs, add the following:
```
tower:
  base_url: https://deploy.domain.ch/api/v2/
  username: user
  password: pass
  parameter_blacklist:
    - unifiedos_creator
  job_templates:
    - id: 11111
    - id: 12345
      validate: metadata.uos_group
```
All variables set in `parameter_blacklist` will be removed from the `extra_vars`.
The list of `job_templates` is a whitelist and only templates included here may be started.
If `validate` is not set, then no further validation will be executed.

**Validations**

Currently only `metadata.uos_group` is supported as a validation.
The validation checks if the users AD groups contains the group defined in `metadata.uos_group`.
The parameter `unifiedos_hostname` must be included in the `extra_vars`.

To add more validations: edit `server/tower/shared.go`

### Route timeout
The `api/aws/ec2` endpoints wait until VMs have the desired state.
This can exceed the default timeout and result in a 504 error on the client.
Increasing the route timeout is described here: https://docs.openshift.org/latest/architecture/networking/routes.html#route-specific-annotations

## The GlusterFS api
Use/see the service unit file in ./glusterapi/install/

### Parameters
```bash
glusterapi -poolName=your-pool -vgName=your-vg -basePath=/your/mount -secret=yoursecret -port=yourport

# poolName = The name of the existing LV-pool that should be used to create new logical volumes
# vgName = The name of the vg where the pool lies on
# basePath = The path where the new volumes should be mounted. E.g. /gluster/mypool
# secret = The basic auth secret you specified above in the SSP
# port = The port where the server should run
# maxGB = Optinally specify max GB a volume can be. Default is 100
```

### Monitoring endpoints
The gluster api has two public endpoints for monitoring purposes. Call them this way:

The first endpoint returns usage statistics:
```bash
curl <yourserver>:<port>/volume/<volume-name>
{"totalKiloBytes":123520,"usedKiloBytes":5472}
```

The check endpoint returns if the current %-usage is below the defined threshold:
```bash

# Successful response
curl -i <yourserver>:<port>/volume/<volume-name>/check\?threshold=20
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Date: Mon, 12 Jun 2017 14:23:53 GMT
Content-Length: 38

{"message":"Usage is below threshold"}

# Error response
curl -i <yourserver>:<port>/volume/<volume-name>/check\?threshold=3

HTTP/1.1 400 Bad Request
Content-Type: application/json; charset=utf-8
Date: Mon, 12 Jun 2017 14:23:37 GMT
Content-Length: 70
{"message":"Error used 4.430051813471502 is bigger than threshold: 3"}
```

For the other (internal) endpoints take a look at the code (glusterapi/main.go)

# Contributing
All required configuration must be set in `config.yaml`. See the `config.yaml.example` file for a sample config.
## Go
```
go run server/main.go
```

## Docker
The backend can be started with Docker.
```
# without proxy:
docker build -p 8000:8000 -t ssp-backend .
# with proxy:
docker build -p 8000:8000 --build-arg https_proxy=http://proxy.ch:9000 -t ssp-backend .

# env_vars must not contain export and quotes
docker run -it --rm ssp-backend
```
