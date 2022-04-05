package pki

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/certutil"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

func Test_ConfigsRoundTrip(t *testing.T) {
	_, s := createBackendWithStorage(t)

	// Verify we handle nothing stored properly
	keyConfigEmpty, err := getKeysConfig(ctx, s)
	require.NoError(t, err)
	require.Equal(t, &keyConfig{}, keyConfigEmpty)

	issuerConfigEmpty, err := getIssuersConfig(ctx, s)
	require.NoError(t, err)
	require.Equal(t, &issuerConfig{}, issuerConfigEmpty)

	// Now attempt to store and reload properly
	origKeyConfig := &keyConfig{
		DefaultKeyId: genKeyId(t),
	}
	origIssuerConfig := &issuerConfig{
		DefaultIssuerId: genIssuerId(t),
	}

	err = setKeysConfig(ctx, s, origKeyConfig)
	require.NoError(t, err)
	err = setIssuersConfig(ctx, s, origIssuerConfig)
	require.NoError(t, err)

	keyConfig, err := getKeysConfig(ctx, s)
	require.NoError(t, err)
	require.Equal(t, origKeyConfig, keyConfig)

	issuerConfig, err := getIssuersConfig(ctx, s)
	require.NoError(t, err)
	require.Equal(t, origIssuerConfig, issuerConfig)
}

func Test_IssuerRoundTrip(t *testing.T) {
	b, s := createBackendWithStorage(t)
	issuer1, key1 := genIssuerAndKey(t, b)
	issuer2, key2 := genIssuerAndKey(t, b)

	// We get an error when issuer id not found
	_, err := fetchIssuerById(ctx, s, issuer1.ID)
	require.Error(t, err)

	// We get an error when key id not found
	_, err = fetchKeyById(ctx, s, key1.ID)
	require.Error(t, err)

	// Now write out our issuers and keys
	err = writeKey(ctx, s, key1)
	require.NoError(t, err)
	err = writeIssuer(ctx, s, issuer1)
	require.NoError(t, err)

	err = writeKey(ctx, s, key2)
	require.NoError(t, err)
	err = writeIssuer(ctx, s, issuer2)
	require.NoError(t, err)

	fetchedKey1, err := fetchKeyById(ctx, s, key1.ID)
	require.NoError(t, err)

	fetchedIssuer1, err := fetchIssuerById(ctx, s, issuer1.ID)
	require.NoError(t, err)

	require.Equal(t, &key1, fetchedKey1)
	require.Equal(t, &issuer1, fetchedIssuer1)

	keys, err := listKeys(ctx, s)
	require.NoError(t, err)

	require.ElementsMatch(t, []keyId{key1.ID, key2.ID}, keys)

	issuers, err := listIssuers(ctx, s)
	require.NoError(t, err)

	require.ElementsMatch(t, []issuerId{issuer1.ID, issuer2.ID}, issuers)
}

func Test_KeysIssuerImport(t *testing.T) {
	b, s := createBackendWithStorage(t)
	issuer1, key1 := genIssuerAndKey(t, b)
	issuer2, key2 := genIssuerAndKey(t, b)

	// Key 1 before Issuer 1; Issuer 2 before Key 2.
	// Remove KeyIDs from non-written entities before beginning.
	key1.ID = ""
	issuer1.ID = ""
	issuer1.KeyID = ""

	key1_ref1, existing, err := importKey(ctx, s, key1.PrivateKey)
	require.NoError(t, err)
	require.False(t, existing)
	require.Equal(t, key1.PrivateKey, key1_ref1.PrivateKey)

	key1_ref2, existing, err := importKey(ctx, s, key1.PrivateKey)
	require.NoError(t, err)
	require.True(t, existing)
	require.Equal(t, key1.PrivateKey, key1_ref1.PrivateKey)
	require.Equal(t, key1_ref1.ID, key1_ref2.ID)

	issuer1_ref1, existing, err := importIssuer(ctx, s, issuer1.Certificate)
	require.NoError(t, err)
	require.False(t, existing)
	require.Equal(t, issuer1.Certificate, issuer1_ref1.Certificate)
	require.Equal(t, key1_ref1.ID, issuer1_ref1.KeyID)

	issuer1_ref2, existing, err := importIssuer(ctx, s, issuer1.Certificate)
	require.NoError(t, err)
	require.True(t, existing)
	require.Equal(t, issuer1.Certificate, issuer1_ref1.Certificate)
	require.Equal(t, issuer1_ref1.ID, issuer1_ref2.ID)
	require.Equal(t, key1_ref1.ID, issuer1_ref2.KeyID)

	err = writeIssuer(ctx, s, issuer2)
	require.NoError(t, err)

	err = writeKey(ctx, s, key2)
	require.NoError(t, err)

	issuer2_ref, existing, err := importIssuer(ctx, s, issuer2.Certificate)
	require.NoError(t, err)
	require.True(t, existing)
	require.Equal(t, issuer2.Certificate, issuer2_ref.Certificate)
	require.Equal(t, issuer2_ref.ID, issuer2.ID)
	require.Equal(t, issuer2_ref.KeyID, issuer2.KeyID)

	key2_ref, existing, err := importKey(ctx, s, key2.PrivateKey)
	require.NoError(t, err)
	require.True(t, existing)
	require.Equal(t, key2.PrivateKey, key2_ref.PrivateKey)
	require.Equal(t, key2_ref.ID, key2.ID)
}

func genIssuerAndKey(t *testing.T, b *backend) (issuer, key) {
	certBundle, err := genCertBundle(t, b)
	require.NoError(t, err)

	keyId := genKeyId(t)

	pkiKey := key{
		ID:             keyId,
		PrivateKeyType: certBundle.PrivateKeyType,
		PrivateKey:     certBundle.PrivateKey,
	}

	issuerId := genIssuerId(t)

	pkiIssuer := issuer{
		ID:           issuerId,
		KeyID:        keyId,
		Certificate:  certBundle.Certificate,
		CAChain:      certBundle.CAChain,
		SerialNumber: certBundle.SerialNumber,
	}

	return pkiIssuer, pkiKey
}

func genIssuerId(t *testing.T) issuerId {
	issuerIdStr, err := uuid.GenerateUUID()
	require.NoError(t, err)
	return issuerId(issuerIdStr)
}

func genKeyId(t *testing.T) keyId {
	keyIdStr, err := uuid.GenerateUUID()
	require.NoError(t, err)
	return keyId(keyIdStr)
}

func genCertBundle(t *testing.T, b *backend) (*certutil.CertBundle, error) {
	// Pretty gross just to generate a cert bundle, but
	fields := addCACommonFields(map[string]*framework.FieldSchema{})
	fields = addCAKeyGenerationFields(fields)
	fields = addCAIssueFields(fields)
	apiData := &framework.FieldData{
		Schema: fields,
		Raw: map[string]interface{}{
			"exported": "internal",
			"cn":       "example.com",
			"ttl":      3600,
		},
	}
	_, _, role, respErr := b.getGenerationParams(ctx, apiData, "/pki")
	require.Nil(t, respErr)

	input := &inputBundle{
		req: &logical.Request{
			Operation: logical.UpdateOperation,
			Path:      "issue/testrole",
			Storage:   b.storage,
		},
		apiData: apiData,
		role:    role,
	}
	parsedCertBundle, err := generateCert(ctx, b, input, nil, true, rand.Reader)

	require.NoError(t, err)
	certBundle, err := parsedCertBundle.ToCertBundle()
	require.NoError(t, err)
	return certBundle, err
}
