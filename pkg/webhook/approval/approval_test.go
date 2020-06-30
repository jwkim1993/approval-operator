package approval

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionRegistrationV1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"approval-operator/internal"
)

func TestCreateCert(t *testing.T) {
	// Get service name and namespace
	svcName := internal.WebhookServiceName()
	ns, err := internal.Namespace()
	if err != nil {
		t.Fatal(err, "error getting namespace")
	}

	// Dummy  validatingwebhookconfigurations
	conf := &admissionRegistrationV1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: ValidationConfigName},
		Webhooks: []admissionRegistrationV1.ValidatingWebhook{
			{
				Name: "validating.approval.tmax.io",
				ClientConfig: admissionRegistrationV1.WebhookClientConfig{
					Service: &admissionRegistrationV1.ServiceReference{
						Name:      svcName,
						Namespace: ns,
					},
				},
			},
		},
	}

	// Fake client
	s := scheme.Scheme
	s.AddKnownTypes(admissionRegistrationV1.SchemeGroupVersion, conf)
	client := fake.NewFakeClientWithScheme(s, []runtime.Object{conf}...)

	// Create cert
	if err := CreateCert(context.TODO(), client); err != nil {
		t.Fatal(err, "Error occurred at CreateCert")
	}

	// Check if server side credentials exist
	keyPath := path.Join(CertDir, "tls.key")
	info, err := os.Stat(keyPath)
	if os.IsNotExist(err) || info.IsDir() {
		t.Fatal(err, "tls.key file is not properly created")
	}
	sKey, err := ioutil.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err, "Error reading tls.key file")
	}

	crtPath := path.Join(CertDir, "tls.crt")
	info, err = os.Stat(crtPath)
	if os.IsNotExist(err) || info.IsDir() {
		t.Fatal(err, "tls.crt file is not properly created")
	}
	serverCertPEM, err := ioutil.ReadFile(crtPath)
	if err != nil {
		t.Fatal(err, "Error reading tls.crt file")
	}

	// Check if CA is saved into validatingwebhookconfigurations
	got := &admissionRegistrationV1.ValidatingWebhookConfiguration{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: ValidationConfigName}, got); err != nil {
		t.Fatal(err, "Cannot get ValidatingWebhookConfiguration")
	}
	if len(got.Webhooks) < 1 {
		t.Fatal("Length of webhooks of retrieved ValidatingWebhookConfiguration is 0")
	}
	caCertBytes := got.Webhooks[0].ClientConfig.CABundle

	// Test if certs are valid
	// Copied from kantive.dev/pkg/webhook/certificates/resources/certs_test.go
	// Test server private key
	p, _ := pem.Decode(sKey)
	if p.Type != "RSA PRIVATE KEY" {
		t.Fatal("Expected the key to be RSA Private key type")
	}
	key, err := x509.ParsePKCS1PrivateKey(p.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key %v", err)
	}
	if err := key.Validate(); err != nil {
		t.Fatalf("Failed to validate private key")
	}

	// Test Server Cert
	sCert, err := validCertificate(serverCertPEM, t)
	if err != nil {
		t.Fatal(err)
	}

	// Test CA Cert
	caParsedCert, err := validCertificate(caCertBytes, t)
	if err != nil {
		t.Fatal(err)
	}

	// Verify domain names
	expectedDNSNames := []string{
		svcName,
		svcName + "." + ns,
		svcName + "." + ns + ".svc",
		svcName + "." + ns + ".svc.cluster.local",
	}
	if diff := cmp.Diff(caParsedCert.DNSNames, expectedDNSNames); diff != "" {
		t.Fatalf("Unexpected CA Cert DNS Name (-want +got) : %v", diff)
	}

	if diff := cmp.Diff(caParsedCert.DNSNames, expectedDNSNames); diff != "" {
		t.Fatalf("Unexpected CA Cert DNS Name (-want +got): %s", diff)
	}

	// Verify Server Cert is Signed by CA Cert
	if err = sCert.CheckSignatureFrom(caParsedCert); err != nil {
		t.Fatal("Failed to verify that the signature on server certificate is from parent CA cert", err)
	}
}

// Copied from kantive.dev/pkg/webhook/certificates/resources/certs_test.go
func validCertificate(cert []byte, t *testing.T) (*x509.Certificate, error) {
	t.Helper()
	const certificate = "CERTIFICATE"
	caCert, _ := pem.Decode(cert)
	if caCert == nil {
		return nil, fmt.Errorf("failed to decode cert")
	}
	if caCert.Type != certificate {
		return nil, fmt.Errorf("cert.Type = %s, want: %s", caCert.Type, certificate)
	}
	parsedCert, err := x509.ParseCertificate(caCert.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cert %w", err)
	}
	if parsedCert.SignatureAlgorithm != x509.SHA256WithRSA {
		return nil, fmt.Errorf("failed to match signature. Got: %s, want: %s", parsedCert.SignatureAlgorithm, x509.SHA256WithRSA)
	}
	return parsedCert, nil
}
