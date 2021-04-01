// Copyright 2021 The Kanister Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kopia

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"sync"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// buffSize is default buffer size for kopia location pull
	bufSize = 65536

	// defaultConfigFilePath is the file which contains kopia repo config
	defaultConfigFilePath = "/tmp/kopia-repository.config"

	// defaultCacheDirectory is the directory where kopia content cache is created
	defaultCacheDirectory = "/tmp/kopia-cache"

	// defaultDataStoreGeneralContentCacheSizeMB is the default content cache size for general command workloads
	defaultDataStoreGeneralContentCacheSizeMB = 0

	// defaultDataStoreGeneralMetadataCacheSizeMB is the default metadata cache size for general command workloads
	defaultDataStoreGeneralMetadataCacheSizeMB = 500

	// TLSCertificateKey represents the key used to fetch the certificate
	// from the secret.
	TLSCertificateKey = "tls.crt"

	// HostNameOption is the key for passing in hostname through ActionSet Options map
	HostNameOption = "hostName"

	// UserNameOption is the key for passing in username through ActionSet Options map
	UserNameOption = "userName"

	// ObjectStorePathOption is the option that specifies the repository to
	// use when describing repo
	ObjectStorePathOption = "objectStorePath"

	// Kopia server info flags
	ServerAddressArg        = "serverAddress"
	UserPassphraseSecretKey = "userPassphraseKey"
	TLSCertSecretKey        = "certs"
)

// ExtractFingerprintFromCertSecret extracts the fingerprint from the given certificate secret
func ExtractFingerprintFromCertSecret(ctx context.Context, cli kubernetes.Interface, secretName, secretNamespace string) (string, error) {
	secret, err := cli.CoreV1().Secrets(secretNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get Certificate Secret. Secret: %s", secretName)
	}

	certBytes, err := json.Marshal(secret.Data[TLSCertificateKey])
	if err != nil {
		return "", errors.Wrap(err, "Failed to marshal Certificate Secret Data")
	}

	var certString string
	if err := json.Unmarshal([]byte(certBytes), &certString); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal Certificate Secret Data")
	}

	decodedCertData, err := base64.StdEncoding.DecodeString(certString)
	if err != nil {
		return "", errors.Wrap(err, "Failed to decode Certificate Secret Data")
	}

	return extractFingerprintFromSliceOfBytes(decodedCertData)
}

// extractFingerprintFromSliceOfBytes extracts the fingeprint from the
// certificate data provided in slice of bytes (default type for secret.Data)
func extractFingerprintFromSliceOfBytes(pemData []byte) (string, error) {
	block, rest := pem.Decode([]byte(pemData))
	if block == nil || len(rest) > 0 {
		return "", errors.New("Failed to PEM Decode Kopia API Server Certificate Secret Data")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse X509 Kopia API Server Certificate Secret Data")
	}

	fingerprint := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(fingerprint[:]), nil
}

// ExtractFingerprintFromCertificateJSON fetch the fingerprint from a base64 encoded,
// certificate which is also type asserted into a string.
func ExtractFingerprintFromCertificateJSON(cert string) (string, error) {
	var certMap map[string]string

	if err := json.Unmarshal([]byte(cert), &certMap); err != nil {
		return "", errors.Wrap(err, "Failed to unmarshal Kopia API Server Certificate Secret Data")
	}

	decodedCertData, err := base64.StdEncoding.DecodeString(certMap[TLSCertificateKey])
	if err != nil {
		return "", errors.Wrap(err, "Failed to base64 decode Kopia API Server Certificate Secret Data")
	}

	fingerprint, err := extractFingerprintFromSliceOfBytes(decodedCertData)
	if err != nil {
		return "", errors.Wrap(err, "Failed to extract fingerprint Kopia API Server Certificate Secret Data")
	}

	return fingerprint, nil
}

// ExtractFingerprintFromCertificate fetch the fingerprint from a base64 encoded,
// certificate which is also type asserted into a string.
func ExtractFingerprintFromCertificate(cert string) (string, error) {
	fmt.Println("TLSCERT::", cert)
	fingerprint, err := extractFingerprintFromSliceOfBytes([]byte(cert))
	if err != nil {
		return "", errors.Wrap(err, "Failed to extract fingerprint Kopia API Server Certificate Secret Data")
	}

	return fingerprint, nil
}

// Location pull copy utility
var bufferPool = sync.Pool{
	New: func() interface{} {
		p := make([]byte, bufSize)
		return &p
	},
}

// Copy is equivalent to io.Copy().
func Copy(dst io.Writer, src io.Reader) (int64, error) {
	bufPtr := bufferPool.Get().(*[]byte)

	defer bufferPool.Put(bufPtr)

	return io.CopyBuffer(dst, src, *bufPtr)
}
