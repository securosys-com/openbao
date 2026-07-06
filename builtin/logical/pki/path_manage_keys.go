// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pki

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"

	"github.com/openbao/go-kms-wrapping/v2/kms"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/helper/certutil"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func pathGenerateKey(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "keys/generate/(internal|exported|kms|external)",

		DisplayAttrs: &framework.DisplayAttributes{
			OperationPrefix: operationPrefixPKI,
			OperationVerb:   "generate",
			OperationSuffix: "internal-key|exported-key|kms-key|external-key",
		},

		Fields: map[string]*framework.FieldSchema{
			keyNameParam: {
				Type:        framework.TypeString,
				Description: "Optional name to be used for this key",
			},
			keyTypeParam: {
				Type:    framework.TypeString,
				Default: "rsa",
				Description: `The type of key to use; defaults to RSA. "rsa"
"ec" and "ed25519" are the only valid values.`,
				AllowedValues: []interface{}{"rsa", "ec", "ed25519"},
				DisplayAttrs: &framework.DisplayAttributes{
					Value: "rsa",
				},
			},
			keyBitsParam: {
				Type:    framework.TypeInt,
				Default: 0,
				Description: `The number of bits to use. Allowed values are
0 (universal default); with rsa key_type: 2048 (default), 3072, or
4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with
ed25519.`,
			},
			"external_config_name": {
				Type:     framework.TypeString,
				Required: false,
			},
			"external_key_options": {
				Type:     framework.TypeMap,
				Required: false,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathGenerateKeyHandler,
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"key_id": {
								Type:        framework.TypeString,
								Description: `ID assigned to this key.`,
								Required:    true,
							},
							"key_name": {
								Type:        framework.TypeString,
								Description: `Name assigned to this key.`,
								Required:    true,
							},
							"key_type": {
								Type: framework.TypeString,
								Description: `The type of key to use; defaults to RSA. "rsa"
								"ec" and "ed25519" are the only valid values.`,
								Required: true,
							},
							"private_key": {
								Type:        framework.TypeString,
								Description: `The private key string`,
								Required:    false,
							},
						},
					}},
				},

				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathGenerateKeyHelpSyn,
		HelpDescription: pathGenerateKeyHelpDesc,
	}
}

const (
	pathGenerateKeyHelpSyn  = `Generate a new private key used for signing.`
	pathGenerateKeyHelpDesc = `This endpoint will generate a new key pair of the specified type (internal, exported, or kms).`
)

func (b *backend) pathGenerateKeyHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// Since we're planning on updating issuers here, grab the lock so we've
	// got a consistent view.
	b.issuersLock.Lock()
	defer b.issuersLock.Unlock()

	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Can not generate keys until migration has completed"), nil
	}

	sc := b.makeStorageContext(ctx, req.Storage)
	keyName, err := getKeyName(sc, data)
	if err != nil { // Fail Immediately if Key Name is in Use, etc...
		return logical.ErrorResponse(err.Error()), nil
	}

	exportPrivateKey := false
	var keyBundle certutil.KeyBundle
	var actualPrivateKeyType certutil.PrivateKeyType
	switch {
	case strings.HasSuffix(req.Path, "/external"):
		configName := data.Get("external_config_name").(string)
		keyType := certutil.PrivateKeyType(data.Get(keyTypeParam).(string))
		externalKeyOptions, _ := data.Get("external_key_options").(map[string]interface{})

		key, exists, err := sc.importExternalKeyReference(
			keyName,
			keyType,
			configName,
			externalKeyOptions,
		)
		if err != nil {
			return logical.ErrorResponse(err.Error()), nil
		}
		if exists {
			return logical.ErrorResponse("Key already exists, use key/ endpoint to update name."), nil
		}

		return &logical.Response{
			Data: map[string]interface{}{
				keyIdParam:   key.ID,
				keyNameParam: key.Name,
				keyTypeParam: keyType,
			},
		}, nil
	case strings.HasSuffix(req.Path, "/exported"):
		exportPrivateKey = true
		fallthrough
	case strings.HasSuffix(req.Path, "/internal"):
		keyType := data.Get(keyTypeParam).(string)
		keyBits := data.Get(keyBitsParam).(int)

		keyBits, _, err := certutil.ValidateDefaultOrValueKeyTypeSignatureLength(keyType, keyBits, 0)
		if err != nil {
			return logical.ErrorResponse("Validation for key_type, key_bits failed: %s", err.Error()), nil
		}

		// Internal key generation, stored in storage
		keyBundle, err = certutil.CreateKeyBundle(keyType, keyBits, b.GetRandomReader())
		if err != nil {
			return nil, err
		}

		actualPrivateKeyType = keyBundle.PrivateKeyType
	default:
		return logical.ErrorResponse("Unknown type of key to generate"), nil
	}

	privateKeyPemString, err := keyBundle.ToPrivateKeyPemString()
	if err != nil {
		return nil, err
	}

	key, _, err := sc.importKey(privateKeyPemString, keyName, keyBundle.PrivateKeyType)
	if err != nil {
		return nil, err
	}
	responseData := map[string]interface{}{
		keyIdParam:   key.ID,
		keyNameParam: key.Name,
		keyTypeParam: string(actualPrivateKeyType),
	}
	if exportPrivateKey {
		responseData["private_key"] = privateKeyPemString
	}
	return &logical.Response{
		Data: responseData,
	}, nil
}

func pathImportKey(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "keys/import",

		DisplayAttrs: &framework.DisplayAttributes{
			OperationPrefix: operationPrefixPKI,
			OperationVerb:   "import",
			OperationSuffix: "key",
		},

		Fields: map[string]*framework.FieldSchema{
			keyNameParam: {
				Type:        framework.TypeString,
				Description: "Optional name to be used for this key",
			},
			"pem_bundle": {
				Type:        framework.TypeString,
				Description: `PEM-format, unencrypted secret key`,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathImportKeyHandler,
				Responses: map[int][]framework.Response{
					http.StatusOK: {{
						Description: "OK",
						Fields: map[string]*framework.FieldSchema{
							"key_id": {
								Type:        framework.TypeString,
								Description: `ID assigned to this key.`,
								Required:    true,
							},
							"key_name": {
								Type:        framework.TypeString,
								Description: `Name assigned to this key.`,
								Required:    true,
							},
							"key_type": {
								Type: framework.TypeString,
								Description: `The type of key to use; defaults to RSA. "rsa"
								"ec" and "ed25519" are the only valid values.`,
								Required: true,
							},
						},
					}},
				},
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathImportKeyHelpSyn,
		HelpDescription: pathImportKeyHelpDesc,
	}
}

const (
	pathImportKeyHelpSyn  = `Import the specified key.`
	pathImportKeyHelpDesc = `This endpoint allows importing a specified issuer key from a pem bundle.
If key_name is set, that will be set on the key, assuming the key did not exist previously.`
)

func (b *backend) pathImportKeyHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// Since we're planning on updating issuers here, grab the lock so we've
	// got a consistent view.
	b.issuersLock.Lock()
	defer b.issuersLock.Unlock()

	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot import keys until migration has completed"), nil
	}

	sc := b.makeStorageContext(ctx, req.Storage)
	pemBundle := data.Get("pem_bundle").(string)
	keyName, err := getKeyName(sc, data)
	if err != nil {
		return logical.ErrorResponse(err.Error()), nil
	}

	if len(pemBundle) < 64 {
		// It is almost nearly impossible to store a complete key in
		// less than 64 bytes. It is definitely impossible to do so when PEM
		// encoding has been applied. Detect this and give a better warning
		// than "provided PEM block contained no data" in this case. This is
		// because the PEM headers contain 5*4 + 6 + 4 + 2 + 2 = 34 characters
		// minimum (five dashes, "BEGIN" + space + at least one character
		// identifier, "END" + space + at least one character identifier, and
		// a pair of new lines). That would leave 30 bytes for Base64 data,
		// meaning at most a 22-byte DER key. Even with a 128-bit key, 6 bytes
		// is not sufficient for the required ASN.1 structure and OID encoding.
		//
		// However, < 64 bytes is probably a good length for a file path so
		// suggest that is the case.
		return logical.ErrorResponse("provided data for import was too short; perhaps a path was passed to the API rather than the contents of a PEM file"), nil
	}

	pemBytes := []byte(pemBundle)
	var pemBlock *pem.Block

	var keys []string
	blockCounter := 0
	for len(bytes.TrimSpace(pemBytes)) > 0 {
		blockCounter++
		pemBlock, pemBytes = pem.Decode(pemBytes)
		if pemBlock == nil {
			msg := fmt.Sprintf("error when parsing block %d: invalid PEM data", blockCounter)
			return logical.ErrorResponse(msg), nil
		}

		pemBlockString := string(pem.EncodeToMemory(pemBlock))
		keys = append(keys, pemBlockString)
	}

	if len(keys) != 1 {
		return logical.ErrorResponse("only a single key can be present within the pem_bundle for importing"), nil
	}

	key, existed, err := importKeyFromBytes(sc, keys[0], keyName)
	if err != nil {
		return logical.ErrorResponse(err.Error()), nil
	}

	resp := logical.Response{
		Data: map[string]interface{}{
			keyIdParam:   key.ID,
			keyNameParam: key.Name,
			keyTypeParam: key.PrivateKeyType,
		},
	}

	if existed {
		resp.AddWarning("Key already imported, use key/ endpoint to update name.")
	}

	return &resp, nil
}

func pathExternalConfig(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "keys/external/config/" + framework.GenericNameRegex("name"),

		DisplayAttrs: &framework.DisplayAttributes{
			OperationPrefix: operationPrefixPKI,
			OperationVerb:   "configure",
			OperationSuffix: "external-key-provider",
		},

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "External key provider config name.",
			},
			"provider": {
				Type:        framework.TypeString,
				Description: "External key provider type, e.g. securosys-hsm.",
			},
			"config": {
				Type:        framework.TypeMap,
				Description: "Provider-specific configuration.",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathExternalConfigWrite,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathExternalConfigRead,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathExternalConfigDelete,
			},
		},

		HelpSynopsis:    "Configure external key provider.",
		HelpDescription: "Stores configuration used by external/HSM-backed PKI keys.",
	}
}
func pathExternalConfigList(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "keys/external/config/?$",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathListOperation,
			},
		},

		HelpSynopsis:    "Configure external key provider.",
		HelpDescription: "Stores configuration used by external/HSM-backed PKI keys.",
	}
}
func (b *backend) pathListOperation(
	ctx context.Context,
	req *logical.Request,
	data *framework.FieldData,
) (*logical.Response, error) {
	sc := b.makeStorageContext(ctx, req.Storage)

	entries, err := sc.fetchListExternalConfigs()
	if err != nil {
		return nil, err
	}

	var responseConfigs []string
	responseInfo := make(map[string]interface{})

	for _, name := range entries {
		config, err := sc.fetchExternalConfig(name)
		if err != nil {
			return nil, err
		}

		responseConfigs = append(responseConfigs, name)
		responseInfo[name] = map[string]interface{}{
			"name":     config.Name,
			"provider": config.Provider,
		}

	}
	return logical.ListResponseWithInfo(responseConfigs, responseInfo), nil
}

func (b *backend) pathExternalConfigWrite(
	ctx context.Context,
	req *logical.Request,
	data *framework.FieldData,
) (*logical.Response, error) {
	sc := b.makeStorageContext(ctx, req.Storage)

	name := data.Get("name").(string)
	provider := data.Get("provider").(string)

	if name == "" {
		return logical.ErrorResponse("name is required"), nil
	}
	if provider == "" {
		return logical.ErrorResponse("provider is required"), nil
	}

	configRaw, ok := data.GetOk("config")
	if !ok {
		return logical.ErrorResponse("config is required"), nil
	}

	config, ok := configRaw.(map[string]interface{})
	if !ok {
		return logical.ErrorResponse("config must be a map"), nil
	}

	entry := &kmsConfigEntry{
		Name:     name,
		Provider: provider,
		Config:   config,
	}

	kmsClient, err := sc.initKmsClient(entry.Provider)
	if err != nil {
		return nil, err
	}
	configMap := externalKMSConfigMap(entry.Provider, entry.Config)
	if err := kmsClient.Open(ctx, &kms.OpenOptions{
		ConfigMap: configMap,
	}); err != nil {
		return nil, err
	}

	if err := sc.writeExternalConfig(entry); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"name":     entry.Name,
			"provider": entry.Provider,
		},
	}, nil
}
func (b *backend) pathExternalConfigRead(
	ctx context.Context,
	req *logical.Request,
	data *framework.FieldData,
) (*logical.Response, error) {
	sc := b.makeStorageContext(ctx, req.Storage)

	name := data.Get("name").(string)
	entry, err := sc.fetchExternalConfig(name)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"name":     entry.Name,
			"provider": entry.Provider,
			"config":   entry.Config,
		},
	}, nil
}
func (b *backend) pathExternalConfigDelete(
	ctx context.Context,
	req *logical.Request,
	data *framework.FieldData,
) (*logical.Response, error) {
	sc := b.makeStorageContext(ctx, req.Storage)

	name := data.Get("name").(string)
	if name == "" {
		return logical.ErrorResponse("name is required"), nil
	}

	if err := sc.deleteExternalConfig(name); err != nil {
		return nil, err
	}

	return nil, nil
}
