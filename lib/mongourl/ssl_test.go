package mongourl

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"unsafe"

	"github.com/globalsign/mgo"
)

func TestSSLVerificationFail(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer server.Close()

	parsedURL, err := Parse("mongodb://someserver?ssl=true")
	if err != nil {
		t.Fatalf("Parse failed")
	}

	// We expect Dial() to fail, because we haven't trusted the server's cert
	_, err = parsedURL.DialServer(getServerFromURL(server.URL))

	if err == nil {
		t.Errorf("Expected DialServer to fail, but it did not")
	}
}

func TestSSLVerificationSuccess(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer server.Close()

	parsedURL, err := Parse("mongodb://someserver?ssl=true")
	if err != nil {
		t.Fatalf("Parse failed")
	}

	// Modify the TLS config we're using to trust our server's self-signed cert
	certificate, _ := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	certpool := x509.NewCertPool()
	certpool.AddCert(certificate)

	tlsConfig = &tls.Config{
		RootCAs: certpool,
	}
	defer func() {
		tlsConfig = &tls.Config{}
	}()

	// We expect Dial() to succeed
	_, err = parsedURL.DialServer(getServerFromURL(server.URL))

	if err != nil {
		t.Errorf("Got unexpected error: %s", err)
	}
}

// Helper to get just the host (e.g. localhost:1234) from a URL
// (e.g. https://localhost:1234). Panics if the URL is invalid.
func getServerFromURL(urlStr string) *mgo.ServerAddr {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		panic(err)
	}

	host := parsedURL.Host

	serverAddr := &mgo.ServerAddr{}

	// It's not possible to construct a ServerAddr outside of the mgo package
	// itself without doing this unsafe reflection
	pointerVal := reflect.ValueOf(serverAddr)
	val := reflect.Indirect(pointerVal)

	member := val.FieldByName("str")
	ptrToY := unsafe.Pointer(member.UnsafeAddr())
	realPtrToY := (*string)(ptrToY)
	*realPtrToY = host

	return serverAddr
}
