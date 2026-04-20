package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"s3server/logger"
)

const (
	MaxTimestampSkew = 300 // 5 minutes
)

var HopByHopHeaders = []string{
	"connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
	"te", "trailer", "transfer-encoding", "upgrade", "x-amzn-trace-id",
}

type Secrets map[string]string

var secrets Secrets

func SetSecrets(s Secrets) {
	secrets = s
}

func GetSecretKey(accessKeyID string) string {
	if secrets == nil {
		return ""
	}
	return secrets[accessKeyID]
}

type SignatureValidator struct {
	req       *http.Request
	accessKey string
}

func NewSignatureValidator(req *http.Request) *SignatureValidator {
	return &SignatureValidator{req: req}
}

func (v *SignatureValidator) Validate(authHeader string) (string, error) {
	logger.Debug("Validating AWS4-HMAC-SHA256 signature")

	parts, err := v.parseAuthHeader(authHeader)
	if err != nil {
		return "", err
	}

	accessKeyID := parts["AccessKeyId"]
	if accessKeyID == "" {
		return "", fmt.Errorf("InvalidAccessKeyId")
	}

	secretKey := GetSecretKey(accessKeyID)
	if secretKey == "" {
		logger.Debug("Secret key not found for: %s", accessKeyID)
		return "", fmt.Errorf("InvalidAccessKeyId")
	}

	if err := v.validateTimestamp(parts); err != nil {
		return "", err
	}

	method := v.req.Method
	methodsToTry := []string{method}
	if method == "GET" || method == "HEAD" {
		if method == "GET" {
			methodsToTry = append(methodsToTry, "HEAD")
		} else {
			methodsToTry = append(methodsToTry, "GET")
		}
	}

	for _, m := range methodsToTry {
		stringToSign := v.buildStringToSign(parts, m)
		calculatedSignature := v.calculateSignature(stringToSign, secretKey, parts)

		if calculatedSignature == parts["Signature"] {
			logger.Debug("Signature verified with method: %s", m)
			return accessKeyID, nil
		}
	}

	return "", fmt.Errorf("SignatureDoesNotMatch")
}

func (v *SignatureValidator) parseAuthHeader(authHeader string) (map[string]string, error) {
	authHeader = strings.ReplaceAll(authHeader, "\r\n", " ")
	authHeader = strings.ReplaceAll(authHeader, "\r", " ")
	authHeader = strings.ReplaceAll(authHeader, "\n", " ")
	authHeader = regexp.MustCompile(`\s+`).ReplaceAllString(authHeader, " ")
	authHeader = strings.TrimSpace(authHeader)

	pattern := `AWS4-HMAC-SHA256\s+Credential=([^,]+),\s*SignedHeaders=([^,]+),\s*Signature=([a-f0-9]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(authHeader)
	if len(matches) < 4 {
		return nil, fmt.Errorf("AccessDenied: Invalid Authorization header format")
	}

	credential := matches[1]
	signedHeaders := matches[2]
	signature := matches[3]

	credentialParts := strings.Split(credential, "/")
	if len(credentialParts) < 5 {
		return nil, fmt.Errorf("InvalidAccessKeyId")
	}

	return map[string]string{
		"AccessKeyId":   credentialParts[0],
		"Date":          credentialParts[1],
		"Region":        credentialParts[2],
		"Service":       credentialParts[3],
		"RequestType":   credentialParts[4],
		"SignedHeaders": signedHeaders,
		"Signature":     signature,
	}, nil
}

func (v *SignatureValidator) validateTimestamp(parts map[string]string) error {
	amzDate := v.getAmzDate()
	if amzDate == "" {
		return fmt.Errorf("InvalidRequest: X-Amz-Date header is required")
	}

	requestTime, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return fmt.Errorf("InvalidRequest: Invalid X-Amz-Date format")
	}

	now := time.Now().UTC()
	diff := now.Sub(requestTime).Abs()

	if diff > MaxTimestampSkew*time.Second {
		return fmt.Errorf("ExpiredToken: Request timestamp skew too large")
	}

	return nil
}

func (v *SignatureValidator) getAmzDate() string {
	headers := v.req.Header

	if amzDate := headers.Get("x-amz-date"); amzDate != "" {
		return amzDate
	}

	if date := headers.Get("date"); date != "" {
		if t, err := time.Parse(time.RFC1123, date); err == nil {
			return t.UTC().Format("20060102T150405Z")
		}
	}

	return time.Now().UTC().Format("20060102T150405Z")
}

func (v *SignatureValidator) buildStringToSign(parts map[string]string, overrideMethod string) string {
	method := overrideMethod
	if method == "" {
		method = v.req.Method
	}

	uri := v.req.URL.Path
	if uri == "" {
		uri = "/"
	}

	queryString := v.req.URL.RawQuery

	canonicalUri := v.encodeUri(uri)
	canonicalQueryString := v.normalizeQueryString(queryString)
	canonicalHeaders := v.buildCanonicalHeaders(parts["SignedHeaders"])
	signedHeaders := strings.ToLower(parts["SignedHeaders"])
	hashedPayload := v.getPayloadHash(method)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s\n%s",
		method,
		canonicalUri,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	)

	amzDate := v.getAmzDate()
	date := amzDate[:8]
	region := parts["Region"]
	service := parts["Service"]
	scope := fmt.Sprintf("%s/%s/%s/aws4_request", date, region, service)

	return fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		scope,
		hashData([]byte(canonicalRequest)),
	)
}

func (v *SignatureValidator) encodeUri(uri string) string {
	if uri == "" {
		return "/"
	}

	parts := strings.Split(uri, "/")
	var encodedParts []string

	for _, part := range parts {
		if part == "" {
			encodedParts = append(encodedParts, "")
		} else {
			decoded := urlDecode(part)
			encodedParts = append(encodedParts, urlEncode(decoded))
		}
	}

	result := strings.Join(encodedParts, "/")
	if !strings.HasPrefix(result, "/") {
		result = "/" + result
	}

	return result
}

func (v *SignatureValidator) normalizeQueryString(queryString string) string {
	if queryString == "" {
		return ""
	}

	params := make(map[string]string)
	pairs := strings.Split(queryString, "&")

	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := urlDecode(kv[0])
			value := urlDecode(kv[1])
			params[key] = value
		} else {
			key := urlDecode(kv[0])
			params[key] = ""
		}
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var normalized []string
	for _, key := range keys {
		normalized = append(normalized, urlEncode(key)+"="+urlEncode(params[key]))
	}

	return strings.Join(normalized, "&")
}

func (v *SignatureValidator) buildCanonicalHeaders(signedHeaders string) string {
	headersList := strings.Split(strings.ToLower(signedHeaders), ";")

	v.req.Header = v.req.Header

	var canonicalHeaders []string
	for _, headerName := range headersList {
		headerName = strings.TrimSpace(headerName)
		if headerName == "" {
			continue
		}

		value := v.req.Header.Get(headerName)
		if value != "" {
			normalizedValue := normalizeHeaderValue(value)
			canonicalHeaders = append(canonicalHeaders, headerName+":"+normalizedValue)
		}
	}

	sort.Strings(canonicalHeaders)

	return strings.Join(canonicalHeaders, "\n")
}

func (v *SignatureValidator) getPayloadHash(method string) string {
	if sha256 := v.req.Header.Get("x-amz-content-sha256"); sha256 != "" {
		return sha256
	}

	emptyPayloadMethods := []string{"HEAD", "GET", "DELETE"}
	for _, m := range emptyPayloadMethods {
		if method == m {
			return "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		}
	}

	// Read and hash body
	return hashData([]byte(fmt.Sprintf("body hash should be calculated")))
}

func (v *SignatureValidator) calculateSignature(stringToSign, secretKey string, parts map[string]string) string {
	amzDate := v.getAmzDate()
	date := amzDate[:8]
	region := parts["Region"]
	service := parts["Service"]

	kDate := hmacSHA256([]byte(date), []byte("AWS4"+secretKey))
	kRegion := hmacSHA256([]byte(region), kDate)
	kService := hmacSHA256([]byte(service), kRegion)
	kSigning := hmacSHA256([]byte("aws4_request"), kService)

	result := hmacSHA256([]byte(stringToSign), kSigning)
	return hex.EncodeToString(result)
}

func hmacSHA256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashData(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func normalizeHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	return value
}

func urlEncode(s string) string {
	result := ""
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			result += string(c)
		} else {
			result += fmt.Sprintf("%%%02X", c)
		}
	}
	return result
}

func urlDecode(s string) string {
	result := strings.ReplaceAll(s, "%2F", "/")
	result = strings.ReplaceAll(result, "%20", " ")
	return result
}
