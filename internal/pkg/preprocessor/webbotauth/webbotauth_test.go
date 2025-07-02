package webbotauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestAddHeaders(t *testing.T) {
	// Use RFC 9421 Ed25519 test key
	privateKeyBytes, err := base64.RawURLEncoding.DecodeString("n4Ni-HpISpVObnQMW0wOhCKROaIKqKtW_2ZYb2p9KcU")
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	// Generate the Go ed25519 version of key from bytes
	privateKey := ed25519.NewKeyFromSeed(privateKeyBytes)

	testHost := "example.com"

	// Create a test HTTP request
	reqURL, _ := url.Parse("https://example.com/test")
	req := &http.Request{
		Method: "GET",
		URL:    reqURL,
		Host:   "example.com",
		Header: make(http.Header),
	}

	// Attempt to add the signed headers
	AddHeaders(req, privateKey, testHost)

	// Check that required headers are added
	if req.Header.Get("Signature-Agent") == "" {
		t.Error("Signature-Agent header not added")
	}

	if req.Header.Get("Signature-Input") == "" {
		t.Error("Signature-Input header not added")
	}

	if req.Header.Get("Signature") == "" {
		t.Error("Signature header not added")
	}

	// Verify Signature-Agent format
	signatureAgent := req.Header.Get("Signature-Agent")
	expectedAgent := `"https://example.com/.well-known/http-message-signatures-directory"`
	if signatureAgent != expectedAgent {
		t.Errorf("Expected Signature-Agent %s, got %s", expectedAgent, signatureAgent)
	}

	// Verify Signature-Input contains required components
	signatureInput := req.Header.Get("Signature-Input")
	if !strings.Contains(signatureInput, `("@authority" "signature-agent")`) {
		t.Error("Signature-Input missing required components")
	}
	if !strings.Contains(signatureInput, "created=") {
		t.Error("Signature-Input missing created parameter")
	}
	if !strings.Contains(signatureInput, "expires=") {
		t.Error("Signature-Input missing expires parameter")
	}
	if !strings.Contains(signatureInput, "keyid=") {
		t.Error("Signature-Input missing keyid parameter")
	}
	if !strings.Contains(signatureInput, `alg="ed25519"`) {
		t.Error("Signature-Input missing algorithm parameter")
	}
	if !strings.Contains(signatureInput, `tag="web-bot-auth"`) {
		t.Error("Signature-Input missing tag parameter")
	}

	// Verify Signature format
	signature := req.Header.Get("Signature")
	if !strings.HasPrefix(signature, "sig1=:") || !strings.HasSuffix(signature, ":") {
		t.Error("Signature header not in correct format")
	}

	// Validate the signature
	err = validateSignature(req, privateKey)
	if err != nil {
		t.Errorf("Signature validation failed: %v", err)
	}
}

// validateSignature validates the HTTP message signature in the request
func validateSignature(req *http.Request, privateKey ed25519.PrivateKey) error {
	// Get the public key
	publicKey := privateKey.Public().(ed25519.PublicKey)

	// Parse the Signature header
	signatureHeader := req.Header.Get("Signature")
	if signatureHeader == "" {
		return fmt.Errorf("missing Signature header")
	}

	// Extract signature value (format: sig1=:base64signature:)
	if !strings.HasPrefix(signatureHeader, "sig1=:") || !strings.HasSuffix(signatureHeader, ":") {
		return fmt.Errorf("invalid signature format")
	}

	// Remove "sig1=:" and trailing ":"
	signatureB64 := signatureHeader[6 : len(signatureHeader)-1]
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %v", err)
	}

	// Parse the Signature-Input header
	signatureInputHeader := req.Header.Get("Signature-Input")
	if signatureInputHeader == "" {
		return fmt.Errorf("missing Signature-Input header")
	}

	// Parse signature input parameters
	params, err := parseSignatureInput(signatureInputHeader)
	if err != nil {
		return fmt.Errorf("failed to parse signature input: %v", err)
	}

	// Recreate the signature base
	signatureBase, err := createSignatureBaseFromParams(req, params)
	if err != nil {
		return fmt.Errorf("failed to create signature base: %v", err)
	}

	// Verify the signature
	if !ed25519.Verify(publicKey, []byte(signatureBase), signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

type signatureParams struct {
	components []string
	created    int64
	expires    int64
	keyid      string
	nonce      string
	alg        string
	tag        string
}

// parseSignatureInput parses the Signature-Input header
func parseSignatureInput(input string) (*signatureParams, error) {
	// Example: sig1=("@authority" "signature-agent");created=1735689600;keyid="...";alg="ed25519";expires=1735693200;nonce="...";tag="web-bot-auth"

	// Extract components part
	re := regexp.MustCompile(`sig1=\(([^)]+)\);(.+)`)
	matches := re.FindStringSubmatch(input)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid signature input format")
	}

	componentsStr := matches[1]
	paramsStr := matches[2]

	// Parse components
	components := []string{}
	componentParts := strings.Split(componentsStr, " ")
	for _, part := range componentParts {
		// Remove quotes
		component := strings.Trim(part, `"`)
		components = append(components, component)
	}

	params := &signatureParams{
		components: components,
	}

	// Parse parameters
	paramPairs := strings.Split(paramsStr, ";")
	for _, pair := range paramPairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		value := strings.Trim(kv[1], `"`)

		switch key {
		case "created":
			created, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid created value: %v", err)
			}
			params.created = created
		case "expires":
			expires, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid expires value: %v", err)
			}
			params.expires = expires
		case "keyid":
			params.keyid = value
		case "nonce":
			params.nonce = value
		case "alg":
			params.alg = value
		case "tag":
			params.tag = value
		}
	}

	return params, nil
}

// createSignatureBaseFromParams recreates the signature base string from parsed parameters
func createSignatureBaseFromParams(req *http.Request, params *signatureParams) (string, error) {
	var lines []string

	for _, comp := range params.components {
		switch comp {
		case "@authority":
			// @authority is the Host header value
			authority := req.Host
			if authority == "" {
				authority = req.URL.Host
			}
			lines = append(lines, fmt.Sprintf(`"%s": %s`, comp, authority))
		case "signature-agent":
			// signature-agent is the value of the Signature-Agent header
			signatureAgent := req.Header.Get("Signature-Agent")
			lines = append(lines, fmt.Sprintf(`"%s": %s`, comp, signatureAgent))
		default:
			// Regular header
			headerValue := req.Header.Get(comp)
			lines = append(lines, fmt.Sprintf(`"%s": %s`, comp, headerValue))
		}
	}

	// Add signature parameters - quote the components
	quotedComponents := make([]string, len(params.components))
	for i, comp := range params.components {
		quotedComponents[i] = fmt.Sprintf(`"%s"`, comp)
	}

	lines = append(lines, fmt.Sprintf(`"@signature-params": (%s);created=%d;keyid="%s";alg="%s";expires=%d;nonce="%s";tag="%s"`,
		strings.Join(quotedComponents, " "),
		params.created,
		params.keyid,
		params.alg,
		params.expires,
		params.nonce,
		params.tag))

	return strings.Join(lines, "\n"), nil
}

func TestAddHeadersWithEmptyParams(t *testing.T) {
	req := &http.Request{
		Header: make(http.Header),
	}

	// Test with nil key
	AddHeaders(req, nil, "example.com")
	if len(req.Header) > 0 {
		t.Error("Headers should not be added when key is nil")
	}

	// Test with empty host
	privateKeyBytes, _ := base64.RawURLEncoding.DecodeString("n4Ni-HpISpVObnQMW0wOhCKROaIKqKtW_2ZYb2p9KcU")
	dummyKey := ed25519.NewKeyFromSeed(privateKeyBytes)

	req.Header = make(http.Header) // Reset headers
	AddHeaders(req, dummyKey, "")
	if len(req.Header) > 0 {
		t.Error("Headers should not be added when host is empty")
	}

	// Test with both nil key and empty host
	req.Header = make(http.Header) // Reset headers
	AddHeaders(req, nil, "")
	if len(req.Header) > 0 {
		t.Error("Headers should not be added when both parameters are invalid")
	}
}

func TestCalculateJWKThumbprint(t *testing.T) {
	// Use RFC 9421 Ed25519 test key
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString("JrQLj5P_89iXES9-vFgrIy29clF9CC_oPPsw3c5D0bs")
	if err != nil {
		t.Fatalf("Failed to decode public key: %v", err)
	}

	publicKey := ed25519.PublicKey(publicKeyBytes)

	thumbprint, err := calculateJWKThumbprint(publicKey)
	if err != nil {
		t.Fatalf("Failed to calculate JWK thumbprint: %v", err)
	}

	if thumbprint == "" {
		t.Error("JWK thumbprint should not be empty")
	}
}

func TestGenerateNonce(t *testing.T) {
	nonce1, err := generateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	nonce2, err := generateNonce()
	if err != nil {
		t.Fatalf("Failed to generate second nonce: %v", err)
	}

	if nonce1 == nonce2 {
		t.Error("Nonces should be different")
	}

	if nonce1 == "" || nonce2 == "" {
		t.Error("Nonces should not be empty")
	}
}

func TestParseEd25519PrivateKeyFromJWK(t *testing.T) {
	// RFC 9421 Ed25519 test key in JWK format
	jwkKey := `{
		"kty": "OKP",
		"crv": "Ed25519",
		"kid": "test-key-ed25519",
		"d": "n4Ni-HpISpVObnQMW0wOhCKROaIKqKtW_2ZYb2p9KcU",
		"x": "JrQLj5P_89iXES9-vFgrIy29clF9CC_oPPsw3c5D0bs"
	}`

	// Test parsing JWK format
	privateKey, err := ParseEd25519PrivateKey(jwkKey)
	if err != nil {
		t.Fatalf("Failed to parse JWK: %v", err)
	}

	if privateKey == nil {
		t.Fatal("Private key should not be nil")
	}

	// Verify the public key matches the expected value
	publicKey := privateKey.Public().(ed25519.PublicKey)
	expectedPublicKeyBytes, err := base64.RawURLEncoding.DecodeString("JrQLj5P_89iXES9-vFgrIy29clF9CC_oPPsw3c5D0bs")
	if err != nil {
		t.Fatalf("Failed to decode expected public key: %v", err)
	}

	if !bytes.Equal(publicKey, expectedPublicKeyBytes) {
		t.Errorf("Public key does not match expected value. Got %x, expected %x", publicKey, expectedPublicKeyBytes)
	}

	// Test that we can use the key for signing
	message := []byte("test message")
	signature := ed25519.Sign(privateKey, message)

	// Verify the signature
	if !ed25519.Verify(publicKey, message, signature) {
		t.Error("Signature verification failed")
	}
}

func TestInvalidSignatureValidation(t *testing.T) {
	// Use RFC 9421 Ed25519 test key
	privateKeyBytes, err := base64.RawURLEncoding.DecodeString("n4Ni-HpISpVObnQMW0wOhCKROaIKqKtW_2ZYb2p9KcU")
	if err != nil {
		t.Fatalf("Failed to decode private key: %v", err)
	}

	// Generate the Go ed25519 version of key from bytes
	privateKey := ed25519.NewKeyFromSeed(privateKeyBytes)

	testHost := "example.com"

	// Create a test HTTP request
	reqURL, _ := url.Parse("https://example.com/test")
	req := &http.Request{
		Method: "GET",
		URL:    reqURL,
		Host:   "example.com",
		Header: make(http.Header),
	}

	// Add headers first
	AddHeaders(req, privateKey, testHost)

	// Corrupt the signature
	originalSig := req.Header.Get("Signature")
	corruptedSig := "sig1=:invalid-signature-here:"
	req.Header.Set("Signature", corruptedSig)

	// Validation should fail
	err = validateSignature(req, privateKey)
	if err == nil {
		t.Error("Expected signature validation to fail with corrupted signature, but it passed")
	}

	// Restore original signature - validation should succeed
	req.Header.Set("Signature", originalSig)
	err = validateSignature(req, privateKey)
	if err != nil {
		t.Errorf("Expected signature validation to succeed with correct signature, but got error: %v", err)
	}
}
