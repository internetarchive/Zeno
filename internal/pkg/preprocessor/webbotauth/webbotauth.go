package webbotauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// JWK represents a JSON Web Key for Ed25519
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
}

// JWKPrivate represents a JSON Web Key with private key components
type JWKPrivate struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid,omitempty"`
	D   string `json:"d"` // Private key
	X   string `json:"x"` // Public key
}

func AddHeaders(req *http.Request, WebBotAuthKey ed25519.PrivateKey, WellKnownBotHost string) {
	if WebBotAuthKey == nil || WellKnownBotHost == "" {
		return
	}

	// Calculate JWK thumbprint
	thumbprint, err := calculateJWKThumbprint(WebBotAuthKey.Public().(ed25519.PublicKey))
	if err != nil {
		return
	}

	// Get current time for created and set expires 5 minutes in the future
	now := time.Now().Unix()
	expires := now + 300 // 5 minutes

	// Generate a random nonce
	nonce, err := generateNonce()
	if err != nil {
		return
	}

	// Construct the signature agent URI
	signatureAgent := fmt.Sprintf("https://%s/.well-known/http-message-signatures-directory", WellKnownBotHost)

	// Add the Signature-Agent header
	req.Header.Set("Signature-Agent", fmt.Sprintf(`"%s"`, signatureAgent))

	// Define the components to sign: @authority and signature-agent
	components := []string{"@authority", "signature-agent"}

	// Construct the Signature-Input header
	signatureInput := fmt.Sprintf(`sig1=(%s);created=%d;keyid="%s";alg="ed25519";expires=%d;nonce="%s";tag="web-bot-auth"`,
		strings.Join(quoteComponents(components), " "),
		now,
		thumbprint,
		expires,
		nonce)

	req.Header.Set("Signature-Input", signatureInput)

	// Create the signature base string according to RFC 9421
	signatureBase, err := createSignatureBase(req, components, now, expires, thumbprint, nonce)
	if err != nil {
		return
	}

	// Sign the signature base with the key
	signature := ed25519.Sign(WebBotAuthKey, []byte(signatureBase))

	// Add the Signature header to request
	req.Header.Set("Signature", fmt.Sprintf("sig1=:%s:", base64.StdEncoding.EncodeToString(signature)))
}

// ParseEd25519PrivateKey parses an Ed25519 private key from PEM or JWK format
func ParseEd25519PrivateKey(keyData string) (ed25519.PrivateKey, error) {
	// Try to detect format by checking if it starts with JSON
	trimmed := strings.TrimSpace(keyData)

	if strings.HasPrefix(trimmed, "{") {
		// Looks like JSON (JWK format)
		return ParseEd25519PrivateKeyFromJWK(keyData)
	}

	// Try PEM format
	block, _ := pem.Decode([]byte(keyData))
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("invalid format: not valid PEM or JWK")
	}

	// Parse PKCS#8 private key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS#8 private key: %v", err)
	}

	// Ensure it's an Ed25519 private key
	ed25519Key, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 private key")
	}

	return ed25519Key, nil
}

// ParseEd25519PrivateKeyFromJWK parses an Ed25519 private key from JWK format
func ParseEd25519PrivateKeyFromJWK(keyData string) (ed25519.PrivateKey, error) {
	var jwk JWKPrivate
	err := json.Unmarshal([]byte(keyData), &jwk)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %v", err)
	}

	// Validate key type
	if jwk.Kty != "OKP" || jwk.Crv != "Ed25519" {
		return nil, fmt.Errorf("invalid key type: expected OKP/Ed25519, got %s/%s", jwk.Kty, jwk.Crv)
	}

	// Decode the private key (d parameter)
	privateKeyBytes, err := base64.RawURLEncoding.DecodeString(jwk.D)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %v", err)
	}

	// Ed25519 private key seed must be exactly 32 bytes
	if len(privateKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(privateKeyBytes))
	}

	// Create Ed25519 private key from seed
	privateKey := ed25519.NewKeyFromSeed(privateKeyBytes)
	return privateKey, nil
}

// calculateJWKThumbprint calculates the JWK thumbprint according to RFC 8037
func calculateJWKThumbprint(publicKey ed25519.PublicKey) (string, error) {
	// Create JWK structure
	jwk := JWK{
		Kty: "OKP",
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(publicKey),
	}

	// Marshal to JSON in canonical form (sorted keys)
	jwkBytes, err := json.Marshal(jwk)
	if err != nil {
		return "", err
	}

	// Calculate SHA-256 hash
	hash := sha256.Sum256(jwkBytes)

	// Return base64url-encoded hash
	return base64.RawURLEncoding.EncodeToString(hash[:]), nil
}

// generateNonce generates a random nonce for the signature
func generateNonce() (string, error) {
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(nonce), nil
}

// quoteComponents adds quotes around component names for the signature input
func quoteComponents(components []string) []string {
	quoted := make([]string, len(components))
	for i, comp := range components {
		quoted[i] = fmt.Sprintf(`"%s"`, comp)
	}
	return quoted
}

// createSignatureBase creates the signature base string according to RFC 9421
func createSignatureBase(req *http.Request, components []string, created, expires int64, keyid, nonce string) (string, error) {
	var lines []string

	for _, comp := range components {
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

	// Add signature parameters
	lines = append(lines, fmt.Sprintf(`"@signature-params": (%s);created=%d;keyid="%s";alg="ed25519";expires=%d;nonce="%s";tag="web-bot-auth"`,
		strings.Join(quoteComponents(components), " "),
		created,
		keyid,
		expires,
		nonce))

	return strings.Join(lines, "\n"), nil
}
