# Securosys REST-based HSM integration in OpenBao

This OpenBao update implements a platform-agnostic REST-based HSM interface with zero library installation, while eliminating connectivity hurdles by using secure web connections (TLS). This facilitates the use and deployment in clustered and multi-cloud environments. Moreover, Securosys HSM innovations like hardware enforced multi-authorization are at oneís disposal.
Security is paramount, experience Securosys Primus HSM or CloudsHSM integration without hassle from the very beginning.

- Unlock your Vault with the security of an HSM
- Make use of multi-authorization workflows for compliance applications

This integration is actively maintained by Securosys SA.

## Table of Contents

- [Glossary](#glossary)
- [Setup](#setup)

  - [Environment Variables](#environment-variables)

  - [Additional prerequisites for UI](#additional-prerequisites-for-ui)

- [How to build OpenBao](#how-to-build-openbao)

  - [Using pre-built releases](#using-pre-built-releases)
  - [Build from sources](#build-from-sources)

- [How to run OpenBao](#how-to-run-openbao)

  - [Developer Mode](#developer-mode)
  - [Production mode](#production-mode)
  - [Auto unseal (securosys-hsm)](#auto-unseal-securosys-hsm)
  - [Using Shamir Secrets](#using-shamir-secrets)
  - [OpenBao Server as Docker Image](#openbao-server-as-docker-image)
- [Example of config.hcl](#example-of-config.hcl)
- [Getting Support](#getting-support)
- [License](#license)
---

## Glossary

| Term      | Description                                                                           |
| :-------- | :------------------------------------------------------------------------------------ |
| CloudsHSM | HSM as a service, operated by Securosys                                               |
| HSM       | Hardware Security Module                                                              |
| JSON      | JavaScript Object Notation object                                                     |
| JWT       | JSON Web Token, used to authenticate and authorize users in web applications and APIs |
| SKA       | Smart Key Attributes, attributes adding rules to individual keys                      |
| TSB       | Transaction Security Broker, providing the REST interface                             |
| UI        | User Interface                                                                        |

## Setup

> **Prerequisites:** Install Golang 1.21.2 ([download](https://go.dev/dl/))

- On Windows : Add GOPATH and GOROOT manually to the system environment and restart your console.
  - GOPATH default `%USERPROFILE%\go`
  - GOROOT defaults to `%programfiles%`

### Environment Variables

`export BAO_CLIENT_TIMEOUT=2000` This change is necessary, as the HashiCorp Vault default value is too low (60 seconds). The higher value is required for some operations (e.g. rekey) when waiting for an approved response.

### Additional prerequisites for UI

For the graphical User Interface the following packages must be installed on the machine:

- [Node.js](https://nodejs.org/dist/v16.17.1/) (with NPM) - version v16.17.1
- [Yarn](https://yarnpkg.com/en/) - Can be installed with the command `npm install -g yarn`

## How to build OpenBao

### Using pre-built releases

You can find pre-built releases of the OpenBao on the Securosys JFrog artifactory. Download the latest binary file, corresponding to your target OS, configuration files, or docker image.

Further documentation and credentials are available via the [Securosys Support Portal](https://support.securosys.com/external/knowledge-base/article/192) or the Securosys [web-site](https://www.securosys.com/en/openbao).

### Build from sources

There are multiple ways to build and run the OpenBao application.

To build OpenBao run either
`go build -o [executable_name]`
or use the command `make bin`
This will build it using the **Make file** configuration.
The OpenBao executable will be placed in the **bin** directory.

To build OpenBao with User Interface, run the following commands

- `make static-dist`
- `make bin`
  The OpenBao executable will be placed in the **bin** directory.

To build a docker image, run the following command
`make docker VERSION={$VERSION}` where {$VERSION} will be a version of the Securosys OpenBao image.

To build OpenBao executables for different platforms (Windows, MacOS, Linux, FreeBSD, NetBSD and OpenBSD)
`make release VERSION={$VERSION}` where {$VERSION} will be a version of build.

To build everything, docker image and all executables for all platforms
`make release-all VERSION={$VERSION}` where {$VERSION} will be a version of build.

## How to run Vault CE

You can run OpenBao without building it
`go run ./main.go [openbao parameters]`

### Developer mode

To run the server in **dev mode** use either of the following commands
` go run ./main.go server -dev` or
`./executable_name server -dev`

In dev mode, all data is stored **only in memory** and **keys will be initialized every time**.

---

### Production mode

To run the server in **production mode** use either of the following commands
` go run ./main.go server -config=config.hcl` or
`./executable_name server -config=config.hcl`

Create the directory "**data**" (if it does not already exist).

If the OpenBao server is not yet initialized, then use either of the following commands
`go run ./main.go operator init -address http://127.0.0.1:8200` or
`./executable_name operator init -address http://127.0.0.1:8200`

These commands initialize the OpenBao server with default OpenBao encryption.

---

### Auto-Unseal (securosys-hsm)

Auto-unseal can be achieved via REST (TSB) interface, connected to Securosys Primus HSM or CloudsHSM.
In the configuration file **config.hcl** add the additional seal configuration section **seal "securosys-hsm"**:

```hcl
  seal "securosys-hsm" {
    //Define the unseal key stored on the HSM. Key has to be RSA type and should exists on HSM.
    key_label = "replace-me_key_label"
    //Key password
    key_password = "password"
    //RestApi url for calling requests to Securosys HSM via REST(TSB)
    tsb_api_endpoint = "replace-me_TSB_Endpoint" //https://rest-api.cloudshsm.com, https://sbx-rest-api.cloudshsm.com, https://primusdev.cloudshsm.com
    //Define the authorization type (TOKEN, CERT, NONE)
    auth = "TOKEN"
    //auth = TOKEN: define the JWT token to authorize at TSB
    bearer_token = "replace-me_BearerToken"
    //auth = CERT: mTLS, define the certificate to authorize at TSB
    //cert_path = "replace-me_with_cert_path"
    //Approval checking frequency in seconds
    check_every = 5
    //Wait for user approvals in seconds
    approval_timeout = 30
  }
```
> **Note:** The configuration section **seal securosys-hsm** is only validated on startup of the ** OpenBao Server**.

---

**_Important_**
After the `operator init` command OpenBao will print the Shamir **Unseal Keys** and the **Initial Root Token**:

```
Unseal Key 1: <unseal_key>
...
Initial Root Token: <root_key>
```

Note down these values, and store them in a safe place for disaster recovery.

---

#### Using Shamir Secrets

> **Note:** This works only with normal **Shamir** secrets. Using **seal "securosys-hsm"** the **OpenBao** is automatically unsealed on startup.

**Unseal** the server with the command
`vault operator unseal <unseal_key>`
and write system **env** with root token using this command
`vault login <root-token>`

Alternatively the [Web UI](http://localhost:8200/ui/) can be used.

## OpenBao Server as Docker Image

Prepare the additional configuration files for the docker image:

### File `docker-compose.yml`:

```yml
version: "3.3"
services:
  run:
    container_name: securosys_openbao
    environment:
      - "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    volumes:
      - securosys_openbao_config:/etc/app/config
      - securosys_openbao_db:/etc/app/db
      - securosys_openbao_plugins:/etc/app/plugins
    ports:
      - "0.0.0.0:8200:8200"
      - "0.0.0.0:8201:8201"
    image: securosys.jfrog.io/external-openbao/1.1.1/securosys-openbao:1.1.1.20230918145422
volumes:
  securosys_openbao_config:
    driver: local
    driver_opts:
      o: bind
      type: none
      # Local absolute path to directory which contains all config files
      device: ./config/vault
  securosys_openbao_db:
    driver: local
    driver_opts:
      o: bind
      type: none
      # Local absolute path to directory where we want to store database
      device: ./config/db
  securosys_openbao_plugins:
    driver: local
    driver_opts:
      o: bind
      type: none
      # Local absolute path to directory where are stored custom plugins
      device: ./config/plugins
```

Where **{$version}** has to be replaced with the current version of the docker image.

### File `config.hcl`:

The configuration file differs slightly from the standalone version.

```hcl
//Example of config.hcl for Docker image.
//Addresses or paths are relative to path and addresses inside docker image

storage "raft" {
  path = "/etc/app/db" //Do not change this path
  node_id = "raft_node"
}

listener "tcp" {
  address     = "0.0.0.0:8200" //Do not change this path
  tls_disable = 1
}

disable_mlock=true
plugin_directory="/etc/app/plugins" //Do not change this path
api_addr = "http://0.0.0.0:8200" //Do not change this addr
cluster_addr = "https://127.0.0.1:8201" //Do not change this addr
ui = true

//Add below the config section seal "securosys-hsm" as shown in the auto-unseal chapter
```

## Example of config.hcl

```hcl
//Example of config.hcl for Docker image.
//Addresses or paths are relative to path and addresses inside docker image

storage "raft" {
  path = "/etc/app/db" //Do not change this path
  node_id = "raft_node"
}

listener "tcp" {
  address     = "0.0.0.0:8200" //Do not change this path
  tls_disable = 1
}

disable_mlock=true
plugin_directory="/etc/app/plugins" //Do not change this path
api_addr = "http://0.0.0.0:8200" //Do not change this addr
cluster_addr = "https://127.0.0.1:8201" //Do not change this addr
ui = true

seal "securosys-hsm" {
  //Define the unseal key stored on the HSM. Key has to be RSA type and should exists on HSM.
  key_label = "replace-me_key_label"
  //Key password
  key_password = "password"
  //RestApi url for calling requests to Securosys HSM via REST(TSB)
  tsb_api_endpoint = "replace-me_TSB_Endpoint" //https://rest-api.cloudshsm.com, https://sbx-rest-api.cloudshsm.com, https://primusdev.cloudshsm.com
  //Define the authorization type (TOKEN, CERT, NONE).
  auth = "TOKEN"
  //auth = TOKEN: define the JWT token to authorize at TSB
  bearer_token = "replace-me_BearerToken"
  //auth = CERT: mTLS, define the certificate to authorize at TSB
  cert_path = "replace-me_with_cert_path"
  key_path = "replace-me_with_key_path"
  //Approval checking frequency in seconds.
  check_every = 5
  //Wait for user approvals in seconds.
  approval_timeout = 30

}
```
## Getting Support
**Community Support for Securosys open source software:**
In our Community we welcome contributions. The Community software is open source and community supported, there is no support SLA, but a helpful best-effort Community.

 - To report a problem or suggest a new feature, use the [Issues](https://github.com/securosys-com/hcvault-ce-rest-integration/issues) tab. 

**Commercial Support for REST/TSB and HSM related issues:** 
Securosys customers having an active support contract, open a support ticket via [Securosys Support Portal](https://support.securosys.com/external/service-catalogue/21).

**Getting a temporary CloudsHSM developer account:**
Check-out a time limited developer account by registering [here](https://app.securosys.com) and choosing *Trial Account*.

## License
 Securosys REST-based HSM integration in HashiCorp OpenBao CE is licensed under the HashiCorp Business Source License, please see [LICENSE](https://github.com/securosys-com/hcvault-ce-rest-integration/LICENSE). 
