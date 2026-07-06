// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pki

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/openbao/openbao/sdk/v2/helper/testhelpers/schema"

	"github.com/openbao/openbao/sdk/v2/helper/certutil"

	"github.com/openbao/openbao/sdk/v2/logical"
	"github.com/stretchr/testify/require"
)

func TestPKI_PathManageKeys_GenerateInternalKeys(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)

	tests := []struct {
		name           string
		keyType        string
		keyBits        []int
		wantLogicalErr bool
	}{
		{"all-defaults", "", []int{0}, false},
		{"rsa", "rsa", []int{0, 2048, 3072, 4096}, false},
		{"ec", "ec", []int{0, 224, 256, 384, 521}, false},
		{"ed25519", "ed25519", []int{0}, false},
		{"error-rsa", "rsa", []int{-1, 343444}, true},
		{"error-ec", "ec", []int{-1, 3434324}, true},
		{"error-bad-type", "dskjfkdsfjdkf", []int{0}, true},
	}
	for _, tt := range tests {
		for _, keyBitParam := range tt.keyBits {
			keyName := fmt.Sprintf("%s-%d", tt.name, keyBitParam)
			t.Run(keyName, func(t *testing.T) {
				data := make(map[string]interface{})
				if tt.keyType != "" {
					data["key_type"] = tt.keyType
				}
				if keyBitParam != 0 {
					data["key_bits"] = keyBitParam
				}
				keyName = genUuid() + "-" + tt.keyType + "-key-name"
				data["key_name"] = keyName
				resp, err := b.HandleRequest(t.Context(), &logical.Request{
					Operation:  logical.UpdateOperation,
					Path:       "keys/generate/internal",
					Storage:    s,
					Data:       data,
					MountPoint: "pki/",
				})
				require.NoError(t, err,
					"Failed generating key with values key_type:%s key_bits:%d key_name:%s", tt.keyType, keyBitParam, keyName)
				require.NotNil(t, resp,
					"Got nil response generating key with values key_type:%s key_bits:%d key_name:%s", tt.keyType, keyBitParam, keyName)
				if tt.wantLogicalErr {
					require.True(t, resp.IsError(), "expected logical error but the request passed:\n%#v", resp)
				} else {
					require.False(t, resp.IsError(),
						"Got logical error response when not expecting one, "+
							"generating key with values key_type:%s key_bits:%d key_name:%s\n%s", tt.keyType, keyBitParam, keyName, resp.Error())

					// Special case our all-defaults
					if tt.keyType == "" {
						tt.keyType = "rsa"
					}

					require.Equal(t, tt.keyType, resp.Data["key_type"], "key_type field contained an invalid type")
					require.NotEmpty(t, resp.Data["key_id"], "returned an empty key_id field, should never happen")
					require.Equal(t, keyName, resp.Data["key_name"], "key name was not processed correctly")
					require.Nil(t, resp.Data["private_key"], "private_key field should not appear in internal generation type.")
				}
			})
		}
	}
}

func TestPKI_PathManageKeys_GenerateExportedKeys(t *testing.T) {
	t.Parallel()
	// We tested a lot of the logic above within the internal test, so just make sure we honor the exported contract
	b, s := CreateBackendWithStorage(t)

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/generate/exported",
		Storage:   s,
		Data: map[string]interface{}{
			"key_type": "ec",
			"key_bits": 224,
		},
		MountPoint: "pki/",
	})
	schema.ValidateResponse(t, schema.GetResponseSchema(t, b.Route("keys/generate/exported"), logical.UpdateOperation), resp, true)

	require.NoError(t, err, "Failed generating exported key")
	require.NotNil(t, resp, "Got nil response generating exported key")
	require.Equal(t, "ec", resp.Data["key_type"], "key_type field contained an invalid type")
	require.NotEmpty(t, resp.Data["key_id"], "returned an empty key_id field, should never happen")
	require.Empty(t, resp.Data["key_name"], "key name should have been empty but was not")
	require.NotEmpty(t, resp.Data["private_key"], "private_key field should not be empty in exported generation type.")

	// Make sure we can decode our private key as expected
	keyData := resp.Data["private_key"].(string)
	block, rest := pem.Decode([]byte(keyData))
	require.Empty(t, rest, "should not have had any trailing data")
	require.NotEmpty(t, block, "failed decoding pem block")

	key, err := x509.ParseECPrivateKey(block.Bytes)
	require.NoError(t, err, "failed parsing pem block as ec private key")
	require.Equal(t, elliptic.P224(), key.Curve, "got unexpected curve value in returned private key")
}

func TestPKI_PathManageKeys_GenerateExternalKeyReference(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)
	sc := b.makeStorageContext(t.Context(), s)

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/generate/external",
		Storage:   s,
		Data: map[string]interface{}{
			keyNameParam:           "missing-config-key",
			keyTypeParam:           "rsa",
			"external_config_name": "missing-config",
			"external_key_options": map[string]interface{}{
				"name": "external-rsa-key",
			},
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed generating external key with missing config")
	require.NotNil(t, resp, "got nil response generating external key with missing config")
	require.True(t, resp.IsError(), "expected missing external config to return an error")

	err = sc.writeExternalConfig(&kmsConfigEntry{
		Name:     "test-config",
		Provider: "test-provider",
		Config: map[string]any{
			"endpoint": "test",
		},
	})
	require.NoError(t, err, "failed writing external config")

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/generate/external",
		Storage:   s,
		Data: map[string]interface{}{
			keyNameParam:           "missing-external-key-options",
			keyTypeParam:           "rsa",
			"external_config_name": "test-config",
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed generating external key without external key options")
	require.NotNil(t, resp, "got nil response generating external key without external key options")
	require.True(t, resp.IsError(), "expected missing external_key_options to return an error")

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/generate/external",
		Storage:   s,
		Data: map[string]interface{}{
			keyNameParam:           "missing-external-key-options-name",
			keyTypeParam:           "rsa",
			"external_config_name": "test-config",
			"external_key_options": map[string]interface{}{},
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed generating external key without external key options name")
	require.NotNil(t, resp, "got nil response generating external key without external key options name")
	require.True(t, resp.IsError(), "expected missing external_key_options.name to return an error")

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/generate/external",
		Storage:   s,
		Data: map[string]interface{}{
			keyNameParam:           "external-key-name",
			keyTypeParam:           "rsa",
			"external_config_name": "test-config",
			"external_key_options": map[string]interface{}{
				"name":     "external-rsa-key",
				"password": "test-password",
			},
		},
		MountPoint: "pki/",
	})
	schema.ValidateResponse(t, schema.GetResponseSchema(t, b.Route("keys/generate/external"), logical.UpdateOperation), resp, true)

	require.NoError(t, err, "failed generating external key")
	require.NotNil(t, resp, "got nil response generating external key")
	require.False(t, resp.IsError(), "got error response generating external key: %#v", resp.Error())
	require.Equal(t, certutil.RSAPrivateKey, resp.Data[keyTypeParam])
	require.Equal(t, "external-key-name", resp.Data[keyNameParam])
	require.NotEmpty(t, resp.Data[keyIdParam], "returned an empty key_id field")
	require.Nil(t, resp.Data["private_key"], "private_key field should not appear in external generation type")

	keyID := resp.Data[keyIdParam].(keyID)
	key, err := sc.fetchKeyById(keyID)
	require.NoError(t, err, "failed fetching generated external key")
	require.NotNil(t, key.ExternalKey, "generated key did not store external key reference")
	require.Equal(t, "test-config", key.ExternalKey.ConfigName)
	require.Equal(t, "rsa", key.ExternalKey.KeyType)
	require.Equal(t, "external-rsa-key", key.ExternalKey.Options["name"])
	require.Equal(t, "test-password", key.ExternalKey.Options["password"])
	require.Empty(t, key.PrivateKey, "external key should not store private key material")

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/generate/external",
		Storage:   s,
		Data: map[string]interface{}{
			keyNameParam:           "duplicate-external-key-name",
			keyTypeParam:           "rsa",
			"external_config_name": "test-config",
			"external_key_options": map[string]interface{}{
				"name": "external-rsa-key",
			},
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed generating duplicate external key")
	require.NotNil(t, resp, "got nil response generating duplicate external key")
	require.True(t, resp.IsError(), "expected duplicate external key reference to return an error")
}

func TestPKI_PathManageKeys_ExternalKMSConfigUsesSnakeCase(t *testing.T) {
	t.Parallel()

	config := map[string]any{
		"rest_api":             "replace-me_tsb_api_endpoint",
		"auth":                 "TOKEN",
		"bearer_token":         "replace-me_bearer_token",
		"cert_path":            "replace-me_cert_path",
		"key_path":             "replace-me_key_path",
		"application_key_pair": "replace-me_application_key_pair",
		"api_keys":             "replace-me_api_keys",
	}

	configMap := externalKMSConfigMap("securosys-hsm", config)

	require.Equal(t, "replace-me_tsb_api_endpoint", configMap["rest_api"])
	require.Equal(t, "replace-me_bearer_token", configMap["bearer_token"])
	require.Equal(t, "replace-me_cert_path", configMap["cert_path"])
	require.Equal(t, "replace-me_key_path", configMap["key_path"])
	require.Equal(t, "replace-me_application_key_pair", configMap["application_key_pair"])
	require.Equal(t, "replace-me_api_keys", configMap["api_keys"])
	require.NotContains(t, config, "restapi")
	require.NotContains(t, config, "bearertoken")
	require.NotContains(t, config, "applicationKeyPair")
	require.NotContains(t, config, "apiKeys")
}

func TestPKI_PathManageKeys_ExternalKMSConfigSupportsLegacyNames(t *testing.T) {
	t.Parallel()

	config := map[string]any{
		"restapi":            "replace-me_tsb_api_endpoint",
		"bearertoken":        "replace-me_bearer_token",
		"certpath":           "replace-me_cert_path",
		"keypath":            "replace-me_key_path",
		"applicationKeyPair": "replace-me_application_key_pair",
		"apiKeys":            "replace-me_api_keys",
	}

	configMap := externalKMSConfigMap("securosys-hsm", config)

	require.Equal(t, "replace-me_tsb_api_endpoint", configMap["rest_api"])
	require.Equal(t, "replace-me_bearer_token", configMap["bearer_token"])
	require.Equal(t, "replace-me_cert_path", configMap["cert_path"])
	require.Equal(t, "replace-me_key_path", configMap["key_path"])
	require.Equal(t, "replace-me_application_key_pair", configMap["application_key_pair"])
	require.Equal(t, "replace-me_api_keys", configMap["api_keys"])
}

func TestPKI_PathManageKeys_ImportKeyBundle(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)

	bundle1, err := certutil.CreateKeyBundle("ec", 224, rand.Reader)
	require.NoError(t, err, "failed generating an ec key bundle")
	bundle2, err := certutil.CreateKeyBundle("rsa", 2048, rand.Reader)
	require.NoError(t, err, "failed generating an rsa key bundle")
	pem1, err := bundle1.ToPrivateKeyPemString()
	require.NoError(t, err, "failed converting ec key to pem")
	pem2, err := bundle2.ToPrivateKeyPemString()
	require.NoError(t, err, "failed converting rsa key to pem")

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"key_name":   "my-ec-key",
			"pem_bundle": pem1,
		},
		MountPoint: "pki/",
	})

	schema.ValidateResponse(t, schema.GetResponseSchema(t, b.Route("keys/import"), logical.UpdateOperation), resp, true)

	require.NoError(t, err, "Failed importing ec key")
	require.NotNil(t, resp, "Got nil response importing ec key")
	require.False(t, resp.IsError(), "received an error response: %v", resp.Error())
	require.NotEmpty(t, resp.Data["key_id"], "key id for ec import response was empty")
	require.Equal(t, "my-ec-key", resp.Data["key_name"], "key_name was incorrect for ec key")
	require.Equal(t, certutil.ECPrivateKey, resp.Data["key_type"])
	keyId1 := resp.Data["key_id"].(keyID)

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"key_name":   "my-rsa-key",
			"pem_bundle": pem2,
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "Failed importing rsa key")
	require.NotNil(t, resp, "Got nil response importing rsa key")
	require.False(t, resp.IsError(), "received an error response: %v", resp.Error())
	require.NotEmpty(t, resp.Data["key_id"], "key id for rsa import response was empty")
	require.Equal(t, "my-rsa-key", resp.Data["key_name"], "key_name was incorrect for ec key")
	require.Equal(t, certutil.RSAPrivateKey, resp.Data["key_type"])
	keyId2 := resp.Data["key_id"].(keyID)

	require.NotEqual(t, keyId1, keyId2)

	// Attempt to reimport the same key with a different name.
	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"key_name":   "my-new-ec-key",
			"pem_bundle": pem1,
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "Failed importing the same ec key")
	require.NotNil(t, resp, "Got nil response importing the same ec key")
	require.False(t, resp.IsError(), "received an error response: %v", resp.Error())
	require.NotEmpty(t, resp.Data["key_id"], "key id for ec import response was empty")
	// Note we should receive back the original name, not the new updated name.
	require.Equal(t, "my-ec-key", resp.Data["key_name"], "key_name was incorrect for ec key")
	require.Equal(t, certutil.ECPrivateKey, resp.Data["key_type"])
	keyIdReimport := resp.Data["key_id"]
	require.Equal(t, keyId1, keyIdReimport, "the re-imported key did not return the same key id")

	// Make sure we can not reuse an existing name across different keys.
	bundle3, err := certutil.CreateKeyBundle("ec", 224, rand.Reader)
	require.NoError(t, err, "failed generating an ec key bundle")
	pem3, err := bundle3.ToPrivateKeyPemString()
	require.NoError(t, err, "failed converting rsa key to pem")
	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"key_name":   "my-ec-key",
			"pem_bundle": pem3,
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "Failed importing the same ec key")
	require.NotNil(t, resp, "Got nil response importing the same ec key")
	require.True(t, resp.IsError(), "should have received an error response importing a key with a re-used name")

	// Delete the key to make sure re-importing gets another ID
	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.DeleteOperation,
		Path:       "key/" + keyId2.String(),
		Storage:    s,
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed deleting keyId 2")
	require.Nil(t, resp, "Got non-nil response deleting the key: %#v", resp)

	// Deleting a non-existent key should be okay...
	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.DeleteOperation,
		Path:       "key/" + keyId2.String(),
		Storage:    s,
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed deleting keyId 2")
	require.Nil(t, resp, "Got non-nil response deleting the key: %#v", resp)

	// Let's reimport key 2 post-deletion to make sure we re-generate a new key id
	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"key_name":   "my-rsa-key",
			"pem_bundle": pem2,
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "Failed importing rsa key")
	require.NotNil(t, resp, "Got nil response importing rsa key")
	require.False(t, resp.IsError(), "received an error response: %v", resp.Error())
	require.NotEmpty(t, resp.Data["key_id"], "key id for rsa import response was empty")
	require.Equal(t, "my-rsa-key", resp.Data["key_name"], "key_name was incorrect for ec key")
	require.Equal(t, certutil.RSAPrivateKey, resp.Data["key_type"])
	keyId2Reimport := resp.Data["key_id"].(keyID)

	require.NotEqual(t, keyId2, keyId2Reimport, "re-importing key 2 did not generate a new key id")
}

func TestPKI_PathManageKeys_DeleteDefaultKeyWarns(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.UpdateOperation,
		Path:       "keys/generate/internal",
		Storage:    s,
		Data:       map[string]interface{}{"key_type": "ec"},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "Failed generating key")
	require.NotNil(t, resp, "Got nil response generating key")
	require.False(t, resp.IsError(), "resp contained errors generating key: %#v", resp.Error())
	keyId := resp.Data["key_id"].(keyID)

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.DeleteOperation,
		Path:       "key/" + keyId.String(),
		Storage:    s,
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed deleting default key")
	require.NotNil(t, resp, "Got nil response deleting the default key")
	require.False(t, resp.IsError(), "expected no errors deleting default key: %#v", resp.Error())
	require.NotEmpty(t, resp.Warnings, "expected warnings to be populated on deleting default key")
}

func TestPKI_PathManageKeys_DeleteUsedKeyFails(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.UpdateOperation,
		Path:       "issuers/generate/root/internal",
		Storage:    s,
		Data:       map[string]interface{}{"common_name": "test.com"},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "Failed generating issuer")
	require.NotNil(t, resp, "Got nil response generating issuer")
	require.False(t, resp.IsError(), "resp contained errors generating issuer: %#v", resp.Error())
	keyId := resp.Data["key_id"].(keyID)

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.DeleteOperation,
		Path:       "key/" + keyId.String(),
		Storage:    s,
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed deleting key used by an issuer")
	require.NotNil(t, resp, "Got nil response deleting key used by an issuer")
	require.True(t, resp.IsError(), "expected an error deleting key used by an issuer")
}

func TestPKI_PathManageKeys_UpdateKeyDetails(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.UpdateOperation,
		Path:       "keys/generate/internal",
		Storage:    s,
		Data:       map[string]interface{}{"key_type": "ec"},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "Failed generating key")
	require.NotNil(t, resp, "Got nil response generating key")
	require.False(t, resp.IsError(), "resp contained errors generating key: %#v", resp.Error())
	keyId := resp.Data["key_id"].(keyID)

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.UpdateOperation,
		Path:       "key/" + keyId.String(),
		Storage:    s,
		Data:       map[string]interface{}{"key_name": "new-name"},
		MountPoint: "pki/",
	})
	schema.ValidateResponse(t, schema.GetResponseSchema(t, b.Route("key/"+keyId.String()), logical.UpdateOperation), resp, true)

	require.NoError(t, err, "failed updating key with new name")
	require.NotNil(t, resp, "Got nil response updating key with new name")
	require.False(t, resp.IsError(), "unexpected error updating key with new name: %#v", resp.Error())

	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.ReadOperation,
		Path:       "key/" + keyId.String(),
		Storage:    s,
		MountPoint: "pki/",
	})
	schema.ValidateResponse(t, schema.GetResponseSchema(t, b.Route("key/"+keyId.String()), logical.ReadOperation), resp, true)

	require.NoError(t, err, "failed reading key after name update")
	require.NotNil(t, resp, "Got nil response reading key after name update")
	require.False(t, resp.IsError(), "unexpected error reading key: %#v", resp.Error())
	keyName := resp.Data["key_name"].(string)

	require.Equal(t, "new-name", keyName, "failed to update key_name expected: new-name was: %s", keyName)

	// Make sure we do not allow updates to invalid name values
	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation:  logical.UpdateOperation,
		Path:       "key/" + keyId.String(),
		Storage:    s,
		Data:       map[string]interface{}{"key_name": "a-bad\\-name"},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "failed updating key with a bad name")
	require.NotNil(t, resp, "Got nil response updating key with a bad name")
	require.True(t, resp.IsError(), "expected an error updating key with a bad name, but did not get one.")
}

func TestPKI_PathManageKeys_ImportKeyBundleBadData(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"key_name":   "my-ec-key",
			"pem_bundle": "this-is-not-a-pem-bundle",
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "got a 500 error type response from a bad pem bundle")
	require.NotNil(t, resp, "Got nil response importing a bad pem bundle")
	require.True(t, resp.IsError(), "should have received an error response importing a bad pem bundle")

	// Make sure we also bomb on a proper certificate
	bundle := genCertBundle(t, b, s)
	resp, err = b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"pem_bundle": bundle.Certificate,
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "got a 500 error type response from a certificate pem bundle")
	require.NotNil(t, resp, "Got nil response importing a certificate bundle")
	require.True(t, resp.IsError(), "should have received an error response importing a certificate pem bundle")
}

func TestPKI_PathManageKeys_ImportKeyRejectsMultipleKeys(t *testing.T) {
	t.Parallel()
	b, s := CreateBackendWithStorage(t)

	bundle1, err := certutil.CreateKeyBundle("ec", 224, rand.Reader)
	require.NoError(t, err, "failed generating an ec key bundle")
	bundle2, err := certutil.CreateKeyBundle("rsa", 2048, rand.Reader)
	require.NoError(t, err, "failed generating an rsa key bundle")
	pem1, err := bundle1.ToPrivateKeyPemString()
	require.NoError(t, err, "failed converting ec key to pem")
	pem2, err := bundle2.ToPrivateKeyPemString()
	require.NoError(t, err, "failed converting rsa key to pem")

	importPem := pem1 + "\n" + pem2

	resp, err := b.HandleRequest(t.Context(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "keys/import",
		Storage:   s,
		Data: map[string]interface{}{
			"key_name":   "my-ec-key",
			"pem_bundle": importPem,
		},
		MountPoint: "pki/",
	})
	require.NoError(t, err, "got a 500 error type response from a bad pem bundle")
	require.NotNil(t, resp, "Got nil response importing a bad pem bundle")
	require.True(t, resp.IsError(), "should have received an error response importing a pem bundle with more than 1 key")

	ctx := t.Context()
	sc := b.makeStorageContext(ctx, s)
	keys, _ := sc.listKeys()
	for _, keyId := range keys {
		id, _ := sc.fetchKeyById(keyId)
		t.Logf("%s:%s", id.ID, id.Name)
	}
}
