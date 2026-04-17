# Changelog

## (next)

- Bump Go to 1.26.2
- Update dependencies

## v1.0.18

- Bump Go to 1.25.9
- Support if-none-match for the extension list endpoint
- Update dependencies

## v1.0.17

- feat(chart): split image.name into image.registry + image.name
- Support global.priorityClassName
- Update alpine packages in Docker image to address CVEs
- Update dependencies

## v1.0.16

- Update dependencies

## v1.0.15

- Update dependencies

## v1.0.14

- Update dependencies

## v1.0.13

- Update dependencies^

## v1.0.12

- Make path to problems view configurable

## v1.0.11

- Support custom certificates

## v1.0.10

- Updated dependencies

## v1.0.9

- Updated dependencies
- Fix: entity selector needs to be url encoded

## v1.0.8

- Handle event requests asynchronously, to avoid blocking the agent
- Don't use entitySelector for events if the entity could not be found in Dynatrace
- Problem check should ignore empty strings as entity selector
- Don't log every event request
- Update dependencies

## v1.0.7

- update dependencies
- Use uid instead of name for user statement in Dockerfile

## v1.0.6

- Set new `Technology` property in extension description
- Update dependencies (go 1.23)

## v1.0.5

- Update dependencies (go 1.22)

## v1.0.4

 - Update dependencies

## v1.0.3

 - Update dependencies

## v1.0.2

 - Update dependencies
 - Fix warnings `Could not find step infos for step execution ...` in logs

## v1.0.1

 - Update dependencies

## v1.0.0

 - Initial release
