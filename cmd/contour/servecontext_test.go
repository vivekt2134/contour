// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	envoy_v3 "github.com/projectcontour/contour/internal/envoy/v3"
	"github.com/projectcontour/contour/internal/fixture"
	"github.com/projectcontour/contour/pkg/config"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestServeContextProxyRootNamespaces(t *testing.T) {
	tests := map[string]struct {
		ctx  serveContext
		want []string
	}{
		"empty": {
			ctx: serveContext{
				rootNamespaces: "",
			},
			want: nil,
		},
		"blank-ish": {
			ctx: serveContext{
				rootNamespaces: " \t ",
			},
			want: nil,
		},
		"one value": {
			ctx: serveContext{
				rootNamespaces: "projectcontour",
			},
			want: []string{"projectcontour"},
		},
		"multiple, easy": {
			ctx: serveContext{
				rootNamespaces: "prod1,prod2,prod3",
			},
			want: []string{"prod1", "prod2", "prod3"},
		},
		"multiple, hard": {
			ctx: serveContext{
				rootNamespaces: "prod1, prod2, prod3 ",
			},
			want: []string{"prod1", "prod2", "prod3"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.ctx.proxyRootNamespaces()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expected: %q, got: %q", tc.want, got)
			}
		})
	}
}

func TestServeContextTLSParams(t *testing.T) {
	tests := map[string]struct {
		ctx         serveContext
		expecterror bool
	}{
		"tls supplied correctly": {
			ctx: serveContext{
				ServerConfig: ServerConfig{
					caFile:      "cacert.pem",
					contourCert: "contourcert.pem",
					contourKey:  "contourkey.pem",
				},
			},
			expecterror: false,
		},
		"tls partially supplied": {
			ctx: serveContext{
				ServerConfig: ServerConfig{
					contourCert: "contourcert.pem",
					contourKey:  "contourkey.pem",
				},
			},
			expecterror: true,
		},
		"tls not supplied": {
			ctx:         serveContext{},
			expecterror: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.ctx.verifyTLSFlags()
			goterror := err != nil
			if goterror != tc.expecterror {
				t.Errorf("TLS Config: %s", err)
			}
		})
	}
}

// Testdata for this test case can be re-generated by running:
// make gencerts
// cp certs/*.pem cmd/contour/testdata/X/
func TestServeContextCertificateHandling(t *testing.T) {
	tests := map[string]struct {
		serverCredentialsDir string
		clientCredentialsDir string
		expectedServerCert   string
		expectError          bool
	}{
		"successful TLS connection established": {
			serverCredentialsDir: "testdata/1",
			clientCredentialsDir: "testdata/1",
			expectedServerCert:   "testdata/1/contourcert.pem",
			expectError:          false,
		},
		"rotating server credentials returns new server cert": {
			serverCredentialsDir: "testdata/2",
			clientCredentialsDir: "testdata/2",
			expectedServerCert:   "testdata/2/contourcert.pem",
			expectError:          false,
		},
		"rotating server credentials again to ensure rotation can be repeated": {
			serverCredentialsDir: "testdata/1",
			clientCredentialsDir: "testdata/1",
			expectedServerCert:   "testdata/1/contourcert.pem",
			expectError:          false,
		},
		"fail to connect with client certificate which is not signed by correct CA": {
			serverCredentialsDir: "testdata/2",
			clientCredentialsDir: "testdata/1",
			expectedServerCert:   "testdata/2/contourcert.pem",
			expectError:          true,
		},
	}

	// Create temporary directory to store certificates and key for the server.
	configDir, err := ioutil.TempDir("", "contour-testdata-")
	checkFatalErr(t, err)
	defer os.RemoveAll(configDir)

	ctx := serveContext{
		ServerConfig: ServerConfig{
			caFile:      filepath.Join(configDir, "CAcert.pem"),
			contourCert: filepath.Join(configDir, "contourcert.pem"),
			contourKey:  filepath.Join(configDir, "contourkey.pem"),
		},
	}

	// Initial set of credentials must be linked into temp directory before
	// starting the tests to avoid error at server startup.
	err = linkFiles("testdata/1", configDir)
	checkFatalErr(t, err)

	// Start a dummy server.
	log := fixture.NewTestLogger(t)
	opts := ctx.grpcOptions(log)
	g := grpc.NewServer(opts...)
	if g == nil {
		t.Error("failed to create server")
	}

	address := "localhost:8001"
	l, err := net.Listen("tcp", address)
	checkFatalErr(t, err)

	go func() {
		err := g.Serve(l)
		checkFatalErr(t, err)
	}()
	defer g.GracefulStop()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Link certificates and key to temp dir used by serveContext.
			err = linkFiles(tc.serverCredentialsDir, configDir)
			checkFatalErr(t, err)
			receivedCert, err := tryConnect(address, tc.clientCredentialsDir)
			gotError := err != nil
			if gotError != tc.expectError {
				t.Errorf("Unexpected result when connecting to the server: %s", err)
			}
			if err == nil {
				expectedCert, err := loadCertificate(tc.expectedServerCert)
				checkFatalErr(t, err)
				assert.Equal(t, receivedCert, expectedCert)
			}
		})
	}
}

func TestTlsVersionDeprecation(t *testing.T) {
	// To get tls.Config for the gRPC XDS server, we need to arrange valid TLS certificates and keys.
	// Create temporary directory to store them for the server.
	configDir, err := ioutil.TempDir("", "contour-testdata-")
	checkFatalErr(t, err)
	defer os.RemoveAll(configDir)

	ctx := serveContext{
		ServerConfig: ServerConfig{
			caFile:      filepath.Join(configDir, "CAcert.pem"),
			contourCert: filepath.Join(configDir, "contourcert.pem"),
			contourKey:  filepath.Join(configDir, "contourkey.pem"),
		},
	}

	err = linkFiles("testdata/1", configDir)
	checkFatalErr(t, err)

	// Get preliminary TLS config from the serveContext.
	log := fixture.NewTestLogger(t)
	preliminaryTLSConfig := ctx.tlsconfig(log)

	// Get actual TLS config that will be used during TLS handshake.
	tlsConfig, err := preliminaryTLSConfig.GetConfigForClient(nil)
	checkFatalErr(t, err)

	assert.Equal(t, tlsConfig.MinVersion, uint16(tls.VersionTLS12))
}

func checkFatalErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// linkFiles creates symbolic link of files in src directory to the dst directory.
func linkFiles(src string, dst string) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	matches, err := filepath.Glob(filepath.Join(absSrc, "*"))
	if err != nil {
		return err
	}

	for _, filename := range matches {
		basename := filepath.Base(filename)
		os.Remove(filepath.Join(dst, basename))
		err := os.Symlink(filename, filepath.Join(dst, basename))
		if err != nil {
			return err
		}
	}

	return nil
}

// tryConnect tries to establish TLS connection to the server.
// If successful, return the server certificate.
func tryConnect(address string, clientCredentialsDir string) (*x509.Certificate, error) {
	clientCert := filepath.Join(clientCredentialsDir, "envoycert.pem")
	clientKey := filepath.Join(clientCredentialsDir, "envoykey.pem")
	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}

	clientConfig := &tls.Config{
		ServerName:         "localhost",
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // nolint:gosec
	}
	conn, err := tls.Dial("tcp", address, clientConfig)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	err = peekError(conn)
	if err != nil {
		return nil, err
	}

	return conn.ConnectionState().PeerCertificates[0], nil
}

func loadCertificate(path string) (*x509.Certificate, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(buf)
	return x509.ParseCertificate(block.Bytes)
}

// peekError is a workaround for TLS 1.3: due to shortened handshake, TLS alert
// from server is received at first read from the socket.
// To receive alert for bad certificate, this function tries to read one byte.
// Adapted from https://golang.org/src/crypto/tls/handshake_client_test.go
func peekError(conn net.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err := conn.Read(make([]byte, 1))
	if err != nil {
		if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
			return err
		}
	}
	return nil
}

func TestParseHTTPVersions(t *testing.T) {
	cases := map[string]struct {
		versions      []config.HTTPVersionType
		parseVersions []envoy_v3.HTTPVersionType
	}{
		"empty": {
			versions:      []config.HTTPVersionType{},
			parseVersions: nil,
		},
		"http/1.1": {
			versions:      []config.HTTPVersionType{config.HTTPVersion1},
			parseVersions: []envoy_v3.HTTPVersionType{envoy_v3.HTTPVersion1},
		},
		"http/1.1+http/2": {
			versions:      []config.HTTPVersionType{config.HTTPVersion1, config.HTTPVersion2},
			parseVersions: []envoy_v3.HTTPVersionType{envoy_v3.HTTPVersion1, envoy_v3.HTTPVersion2},
		},
		"http/1.1+http/2 duplicated": {
			versions: []config.HTTPVersionType{
				config.HTTPVersion1, config.HTTPVersion2,
				config.HTTPVersion1, config.HTTPVersion2},
			parseVersions: []envoy_v3.HTTPVersionType{envoy_v3.HTTPVersion1, envoy_v3.HTTPVersion2},
		},
	}

	for name, testcase := range cases {
		testcase := testcase
		t.Run(name, func(t *testing.T) {
			vers := parseDefaultHTTPVersions(testcase.versions)

			// parseDefaultHTTPVersions doesn't guarantee a stable result, but the order doesn't matter.
			sort.Slice(vers,
				func(i, j int) bool { return vers[i] < vers[j] })
			sort.Slice(testcase.parseVersions,
				func(i, j int) bool { return testcase.parseVersions[i] < testcase.parseVersions[j] })

			assert.Equal(t, testcase.parseVersions, vers)
		})
	}
}
