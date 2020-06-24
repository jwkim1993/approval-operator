package approval

import (
	"context"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"path"
	"strconv"
	"time"

	admissionRegistrationV1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certResources "knative.dev/pkg/webhook/certificates/resources"
)

const (
	DefaultPort          = 443
	CertDir              = "/tmp/approval-webhook"
	Namespace            = "approval"
	ServiceName          = "approval-webhook"
	ValidationPath       = "/validate-approvals"
	ValidationConfigName = "validating.approval.tmax.io"
)

func Port() int {
	envPort := os.Getenv("WEBHOOK_PORT")
	if envPort == "" {
		return DefaultPort
	} else {
		port, err := strconv.Atoi(envPort)
		if err != nil {
			log.Log.Error(err, "Cannot parse port number")
			os.Exit(1)
		}
		return port
	}
}

// Create and Store certificates for webhook server
// server key / server cert is stored as file in CertDir
// CA bundle is stored in ValidatingWebhookConfigurations
func CreateCert(ctx context.Context, client client.Client) error {
	// Make directory recursively
	if err := os.MkdirAll(CertDir, os.ModePerm); err != nil {
		return err
	}

	// Create certs
	tlsKey, tlsCrt, caCrt, err := certResources.CreateCerts(ctx, ServiceName, Namespace, time.Now().AddDate(1, 0, 0))
	if err != nil {
		return err
	}

	// Write certs to file
	keyPath := path.Join(CertDir, "tls.key")
	err = ioutil.WriteFile(keyPath, tlsKey, 0644)
	if err != nil {
		return err
	}

	crtPath := path.Join(CertDir, "tls.crt")
	err = ioutil.WriteFile(crtPath, tlsCrt, 0644)
	if err != nil {
		return err
	}

	// Update validatingWebhookConfigurations
	conf := &admissionRegistrationV1.ValidatingWebhookConfiguration{}
	if err = client.Get(ctx, types.NamespacedName{Name: ValidationConfigName}, conf); err != nil {
		// Return error, even if it is 'not found' error
		// ValidationWebhookConfiguration object should be created at installation time
		return err
	}
	for i := range conf.Webhooks {
		conf.Webhooks[i].ClientConfig.CABundle = caCrt
	}
	if err = client.Update(ctx, conf); err != nil {
		return err
	}

	return nil
}
