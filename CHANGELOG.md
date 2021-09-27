# Changelog

**ssp-backend**

This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
and [human-readable changelog](https://keepachangelog.com/en/1.0.0/).

The current role maintainer is the SBB Cloud Platform Team.

## [Master](https://github.com/SchweizerischeBundesbahnen/ssp-backend/commits/master) - unreleased

### Added

## [3.9.1](https://github.com/SchweizerischeBundesbahnen/ssp-backend/compare/v3.9.1...v3.9.0) - 03.08.2020

### Added

- Fixed the filter on `server/openshift/project.go` for the REST call `api/ose/projects`, so it
  filters if the parameter is specified, even as an empty string, but does not filter if the
  parameter is not specified (the behavior in 3.9.0 was that if the parameter was not present, it
  interpreted it as an empty string)

## [3.9.0](https://github.com/SchweizerischeBundesbahnen/ssp-backend/compare/v3.9.0...v3.8.1) - 29.07.2020

### Added

- Function `getJobTemplateDetails()` and API route `/tower/job_templates/<id>/getDetails` (GET)
  added, to get survey specs from Ansible Tower templates.
- The "functional account" (an additional project admin for Openshift) is now configurable and not
  hardcoded into the go file.
- New tests for the validateProjectPermissions() function from openshift/project.go

## [3.8.1]

Changes were not tracked...
