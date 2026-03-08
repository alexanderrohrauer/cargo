# Cargo
Cargo is a GitOps tool for docker compose written in Go. It is a CLI application that comes with a REST API.
The REST API is used to communicate with the CLI. One installation is running as a compose project on the server (as REST server - startable with a CLI command) whereas one installation is running on the dev-device that communicates with the REST API.

## Features
- Automatic generation of SSL certificates for HTTPS usage in the REST API
- Configuration of compose projects via YML file (the folder for the docker-compose.yml in the Git repository should be specifyable)
- The Git revision should be specifyable in the Configuration
- The tool should use SOPS in the background for encrypting env secrets for the compose project  (also generate the keys for sops)
- The tool should use authentication for the REST API via a static auth token that is printed in STD out
- Besides manual syncing of the compose projects, it should support polling for the projects.
- The tool should be able to work with private Git repositories (by providing auth-tokens/certificates)
- The tool should have a workdir, where all the projects (so just the folders with the docker-compose.yml including all the secret envs etc.) are stored/cloned.
- There should be a Dockerfile for containerization of the server application

## Dependencies
Use the following dependencies:
- Git library
- sops
- docker compose sdk
- spf13/cobra
- ...you can also use other packages that are useful.

## Quality requirements
Please make sure, that the code is maintainable and well structured. 