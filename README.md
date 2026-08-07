# Securosys REST-based HSM integration in OpenBao

This is a fork of [OpenBao](https://github.com/openbao/openbao) which
adds improved support for Securosys HSMs.

In particular, this fork adds support for accessing a Primus HSM through the
REST API exposed by the [Securosys Transaction Security Broker (TSB)](https://docs.securosys.com/tsb/overview/).
This has the following benefits:

- Auto-unseal: Unlock your Vault with the security of an HSM.
- PKI external keys: Use the OpenBao PKI features with keys stored in an HSM.
- Multi-authorization: With the TSB, it is easy to use [Smart Key Attributes (SKA)](https://docs.securosys.com/ska/overview) in your OpenBao Vault.
- No library installation: Unlike PKCS#11, REST does not require the installation of a dynamic library or additional config files.

This integration is actively maintained by Securosys SA.

## Table of Contents

- [Glossary](#glossary)
- [Setup](#setup)
  - [Additional prerequisites for UI](#additional-prerequisites-for-ui)

- [How to build OpenBao](#how-to-build-openbao)
  - [Using pre-built releases](#using-pre-built-releases)
  - [Build from sources](#build-from-sources)

- [How to run OpenBao](#how-to-run-openbao)
  - [Developer Mode](#developer-mode)
  - [Production mode](#production-mode)
  - [Auto unseal (securosys-hsm)](#auto-unseal-securosys-hsm)
  - [Self-initialization with HCL files](#self-initialization-with-hcl-files)
  - [PKI external keys](#pki-external-keys)

- [Examples with three HCL files](#examples-with-three-hcl-files)
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

## How to build OpenBao

### Prerequisites

To build OpenBao from source, you need to install:

- [Go](https://go.dev/dl/)
- [NodeJS with pnpm](https://nodejs.org/en/download)

On Windows: Add GOPATH and GOROOT manually to the system environment and restart your console.

- GOPATH default `%USERPROFILE%\go`
- GOROOT defaults to `%programfiles%`

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

To build OpenBao executables for different platforms (Windows, MacOS, Linux, FreeBSD, NetBSD and OpenBSD)
`make release VERSION={$VERSION}` where {$VERSION} will be a version of build.

To build all executables for all platforms
`make release-all VERSION={$VERSION}` where {$VERSION} will be a version of build.

## How to run Vault CE

You can build and run OpenBao in one step with
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

For first-start initialization with auto-unseal, use the self-initialization flow described below.

---

### Auto-Unseal (securosys-hsm)

Auto-unseal can be achieved via REST (TSB) interface, connected to Securosys Primus HSM or CloudsHSM.
In the configuration file **config.hcl** add the additional seal configuration section **seal "securosys-hsm"**:

```hcl
  seal "securosys-hsm" {
    //Define the unseal key stored on the HSM. Key has to be RSA type and should exists on HSM.
    key_label = "replace-me_key_label"
    //Key password
    key_password = "replace-me_key_password"
    //TSB API endpoint for REST requests to Securosys HSM
    tsb_api_endpoint = "replace-me_tsb_api_endpoint"
    //Define the authorization type (TOKEN, CERT, NONE)
    auth = "TOKEN"
    //auth = TOKEN: define the JWT token to authorize at TSB
    bearer_token = "replace-me_bearer_token"
    //auth = CERT: mTLS, define the certificate to authorize at TSB
    //cert_path = "replace-me_cert_path"
    //key_path = "replace-me_key_path"
    //Optional application_key_pair used for request signing.
    application_key_pair = <<EOT
{
  "private_key": "replace-me_application_private_key",
  "public_key": "replace-me_application_public_key"
}
EOT
    //Optional api_keys used by TSB.
    api_keys = <<EOT
{
  "key_management_token": ["replace-me_key_management_token"],
  "key_operation_token": ["replace-me_key_operation_token"],
  "approver_token": ["replace-me_approver_token"],
  "service_token": ["replace-me_service_token"],
  "approver_key_management_token": ["replace-me_approver_key_management_token"]
}
EOT
    //Approval checking frequency in seconds
    check_every = 5
    //Wait for user approvals in seconds
    approval_timeout = 600
  }
```

> **Note:** The configuration section **seal securosys-hsm** is only validated on startup of the ** OpenBao Server**.

Auto-unseal timeout settings:

| Parameter            | Default / example                                                  | Description                                                                 |
| :------------------- | :----------------------------------------------------------------- | :-------------------------------------------------------------------------- |
| `check_every`        | Default/example: `5` seconds                                       | How often OpenBao checks the HSM approval status. Must be greater than `0`. |
| `approval_timeout`   | Default/example: `600` seconds (10 minutes)                        | Maximum time to wait for HSM approval. Must be greater than `check_every`.  |
| `http_read_timeout`  | OpenBao default is `30s`; `2000s` in `config/config.hcl`           | Listener read timeout. Increase it for long approval flows.                 |
| `http_write_timeout` | OpenBao default is unlimited (`0`); `2000s` in `config/config.hcl` | Listener write timeout. Increase it for long approval flows.                |

Optional auto-unseal credentials:

| Parameter | Format | Description |
| :-- | :-- | :-- |
| `application_key_pair` | JSON string with `private_key` and `public_key` | Application key pair used by TSB request signing. |
| `api_keys` | JSON string with token arrays | API keys used by TSB. Supported arrays are `key_management_token`, `key_operation_token`, `approver_token`, `service_token`, and `approver_key_management_token`. |

---

### Self-initialization with HCL files

Self-initialization lets OpenBao initialize itself on first startup and then run declarative bootstrap requests from HCL. It requires auto-unseal, because there is no Shamir key output to persist.

The example files are in `config/`:

- `config/config.hcl`: storage, listener, API address, UI, and timeout settings.
- `config/autounseal.hcl`: `seal "securosys-hsm"` configuration.
- `config/selfinitialization.hcl`: bootstrap requests executed after initialization.

Start OpenBao with the whole `config/` directory:

```shell
bao server \
  -config=config/
```

Alternatively, pass each file explicitly:

```shell
bao server \
  -config=config/config.hcl \
  -config=config/autounseal.hcl \
  -config=config/selfinitialization.hcl
```

The `initialize "bootstrap"` block in `config/selfinitialization.hcl` contains ordered `request` blocks. The current example:

- enables the `userpass` auth method,
- creates an `admin` ACL policy,
- creates an `admin` user with the `admin` policy.

Example request block:

```hcl
initialize "bootstrap" {
  request "enable-userpass" {
    operation = "update"
    path      = "sys/auth/userpass"

    data = {
      type = "userpass"
    }
  }
}
```

Self-initialization runs only when the storage backend is not initialized yet. On later starts, OpenBao skips the `initialize` block.

---

### PKI external keys

PKI can use an existing signing key stored in Securosys HSM without importing the private key PEM into OpenBao. Register the external provider configuration first, then register the external key reference. After that, use the returned `key_id` or `key_name` as `key_ref` in the standard PKI endpoints.

Enable PKI if it is not mounted yet:

```shell
bao secrets enable pki
```

Register the external provider configuration through the HTTP API:

```shell
curl \
  --header "X-Vault-Token: replace-me_token" \
  --request POST \
  --data @replace-me_external_config.json \
  replace-me_openbao_addr/v1/pki/keys/external/config/replace-me_external_config_name
```

Required fields:

- `provider`: external key provider name. Supports `securosys-hsm`.
- `config`: provider-specific connection configuration used to open the HSM/KMS client.

Supported `config.auth` values:

- `TOKEN`: use bearer token authorization. Requires `bearer_token`.
- `CERT`: use client certificate authorization. Requires `cert_path` and `key_path`.
- `NONE`: do not send additional authorization data.

Optional config fields are provider-specific. For `securosys-hsm`, the PKI REST API uses `snake_case` field names such as `rest_api`, `bearer_token`, `cert_path`, `key_path`, `application_key_pair`, and `api_keys`.

Use these `snake_case` names in the PKI REST API payload. The PKI module translates them internally before opening the KMS client.

Example `replace-me_external_config.json` with token authorization:

```json
{
  "provider": "replace-me_provider",
  "config": {
    "rest_api": "replace-me_tsb_api_endpoint",
    "auth": "TOKEN",
    "bearer_token": "replace-me_bearer_token"
  }
}
```

Example `replace-me_external_config.json` with token authorization, application key pair, and API keys:

```json
{
  "provider": "replace-me_provider",
  "config": {
    "rest_api": "replace-me_tsb_api_endpoint",
    "auth": "TOKEN",
    "bearer_token": "replace-me_bearer_token",
    "application_key_pair": "{\"private_key\":\"replace-me_application_private_key\",\"public_key\":\"replace-me_application_public_key\"}",
    "api_keys": "{\"key_management_token\":[\"replace-me_key_management_token\"],\"key_operation_token\":[\"replace-me_key_operation_token\"],\"approver_token\":[\"replace-me_approver_token\"],\"service_token\":[\"replace-me_service_token\"],\"approver_key_management_token\":[\"replace-me_approver_key_management_token\"]}"
  }
}
```

In this example, `application_key_pair` and `api_keys` are OpenBao config fields and therefore use `snake_case`. Their values are JSON strings passed to the TSB client and also use `snake_case`.

Example `replace-me_external_config.json` with client certificate authorization:

```json
{
  "provider": "replace-me_provider",
  "config": {
    "rest_api": "replace-me_tsb_api_endpoint",
    "auth": "CERT",
    "cert_path": "replace-me_cert_path",
    "key_path": "replace-me_key_path"
  }
}
```

Example `replace-me_external_config.json` without additional authorization:

```json
{
  "provider": "replace-me_provider",
  "config": {
    "rest_api": "replace-me_tsb_api_endpoint",
    "auth": "NONE"
  }
}
```

Register an existing external key through the HTTP API:

```shell
curl \
  --header "X-Vault-Token: replace-me_token" \
  --request POST \
  --data '{
    "key_name": "replace-me_key_name",
    "key_type": "replace-me_key_type",
    "external_config_name": "replace-me_external_config_name",
    "external_key_options": {
      "name": "replace-me_external_key_name",
      "password": "replace-me_key_password"
    }
  }' \
  replace-me_openbao_addr/v1/pki/keys/generate/external
```

Required fields:

- `external_config_name`: name created under `pki/keys/external/config/<name>`.
- `key_type`: public key type of the external key. Supported values are `rsa`, `ec`, and `ed25519`.
- `external_key_options.name`: existing key label/name in the external provider.

Optional fields:

- `key_name`: OpenBao-local name for this key reference.
- additional keys in `external_key_options`: provider-specific key options, such as password or signing parameters.

The response contains `key_id`, `key_name`, and `key_type`. It does not contain `private_key`, because the private key stays in the HSM.

Use the external key with normal PKI flows:

```shell
curl \
  --header "X-Vault-Token: replace-me_token" \
  --request POST \
  --data '{
    "key_ref": "replace-me_key_name",
    "issuer_name": "replace-me_issuer_name",
    "common_name": "replace-me_common_name",
    "ttl": "replace-me_ttl"
  }' \
  replace-me_openbao_addr/v1/pki/issuers/generate/root/existing
```

From this point, issuing certificates, signing, CRL, OCSP, roles, and issuer management should work like the existing PKI flow, with `key_ref` pointing to the external key reference.

## Examples with three HCL files

Use three separate config files so base server settings, auto-unseal, and bootstrap requests stay independent.

Start OpenBao with the whole `config/` directory:

```shell
bao server \
  -config=config/
```

Or pass each file explicitly:

```shell
bao server \
  -config=config/config.hcl \
  -config=config/autounseal.hcl \
  -config=config/selfinitialization.hcl
```

### `config/config.hcl`

```hcl
storage "raft" {
  path    = "./db"
  node_id = "raft_node_1"
}

listener "tcp" {
  address     = "127.0.0.1:8200"
  tls_disable = 1

  http_read_timeout  = "2000s"
  http_write_timeout = "2000s"
}

api_addr     = "http://127.0.0.1:8200"
cluster_addr = "https://127.0.0.1:8201"
ui           = true
```

### `config/autounseal.hcl`

```hcl
plugin_directory = "./plugins"

seal "securosys-hsm" {
  key_label        = "replace-me_key_label"
  key_password     = "replace-me_key_password"
  tsb_api_endpoint = "replace-me_tsb_api_endpoint"

  # Authorization type: TOKEN, CERT, or NONE.
  auth         = "TOKEN"
  bearer_token = "replace-me_bearer_token"
  # cert_path = "replace-me_cert_path"
  # key_path  = "replace-me_key_path"

  application_key_pair = <<EOT
{
  "private_key": "replace-me_application_private_key",
  "public_key": "replace-me_application_public_key"
}
EOT

  api_keys = <<EOT
{
  "key_management_token": ["replace-me_key_management_token"],
  "key_operation_token": ["replace-me_key_operation_token"],
  "approver_token": ["replace-me_approver_token"],
  "service_token": ["replace-me_service_token"],
  "approver_key_management_token": ["replace-me_approver_key_management_token"]
}
EOT

  check_every      = 5
  approval_timeout = 600
}
```

### `config/selfinitialization.hcl`

```hcl
initialize "bootstrap" {
  request "enable-userpass" {
    operation = "update"
    path      = "sys/auth/userpass"

    data = {
      type = "userpass"
    }
  }

  request "create-admin-policy" {
    operation = "update"
    path      = "sys/policies/acl/admin"

    data = {
      policy = <<EOT
path "*" {
  capabilities = ["create", "read", "update", "delete", "list", "sudo"]
}
EOT
    }
  }

  request "create-admin-user" {
    operation = "update"
    path      = "auth/userpass/users/admin"

    data = {
      password = "replace-me_admin_password"
      policies = ["admin"]
    }
  }
}
```

## Getting Support

**Community Support for Securosys open source software:**
In our Community we welcome contributions. The Community software is open source and community supported, there is no support SLA, but a helpful best-effort Community.

- To report a problem or suggest a new feature, use the [Issues](https://github.com/securosys-com/hcvault-ce-rest-integration/issues) tab.

**Commercial Support for REST/TSB and HSM related issues:**
Securosys customers having an active support contract, open a support ticket via [Securosys Support Portal](https://support.securosys.com/external/service-catalogue/21).

**Getting a temporary CloudHSM developer account:**
Check-out a time limited developer account by registering on [cloud.securosys.com](https://cloud.securosys.com) and choosing _Trial Account_.

## License

Like upstream OpenBao, this fork is licensed under [MPL-2.0](./LICENSE).
