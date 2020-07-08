package approval

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strconv"
	"time"

	admissionRegistrationV1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certResources "knative.dev/pkg/webhook/certificates/resources"

	"approval-operator/internal"
	tmaxv1 "approval-operator/pkg/apis/tmax/v1"
)

const (
	DefaultPort          = 443
	CertDir              = "/tmp/approval-webhook"
	ValidationPath       = "/validate-approvals"
	ValidationConfigName = "validating.approval.tmax.io"
	MutationPath         = "/mutate-approvals"
	MutationConfigName   = "mutating.approval.tmax.io"
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

	// Get service name and namespace
	svc := internal.WebhookServiceName()
	ns, err := internal.Namespace()
	if err != nil {
		return err
	}

	// Create certs
	tlsKey, tlsCrt, caCrt, err := certResources.CreateCerts(ctx, svc, ns, time.Now().AddDate(1, 0, 0))
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
	valConf := &admissionRegistrationV1.ValidatingWebhookConfiguration{}
	if err = client.Get(ctx, types.NamespacedName{Name: ValidationConfigName}, valConf); err != nil {
		// Return error, even if it is 'not found' error
		// ValidationWebhookConfiguration object should be created at installation time
		return err
	}
	for i := range valConf.Webhooks {
		valConf.Webhooks[i].ClientConfig.CABundle = caCrt
	}
	if err = client.Update(ctx, valConf); err != nil {
		return err
	}

	// Update mutatingWebhookConfigurations
	mutConf := &admissionRegistrationV1.MutatingWebhookConfiguration{}
	if err = client.Get(ctx, types.NamespacedName{Name: MutationConfigName}, mutConf); err != nil {
		// Return error, even if it is 'not found' error
		// MutatingWebhookConfiguration object should be created at installation time
		return err
	}
	for i := range mutConf.Webhooks {
		mutConf.Webhooks[i].ClientConfig.CABundle = caCrt
	}
	if err = client.Update(ctx, mutConf); err != nil {
		return err
	}

	return nil
}

func difference(approvers []tmaxv1.Approver, oldApprovers []tmaxv1.Approver) []tmaxv1.Approver {
	var result []tmaxv1.Approver

	// Find added
	for _, a1 := range approvers {
		found := false
		for _, a2 := range oldApprovers {
			if a1.UserID == a2.UserID {
				found = true
				// Check if changed
				if !reflect.DeepEqual(a1, a2) {
					result = append(result, a1)
				}
				break
			}
		}
		if !found {
			result = append(result, a1)
		}
	}

	// Find deleted
	for _, a2 := range oldApprovers {
		found := false
		for _, a1 := range approvers {
			if a2.UserID == a1.UserID {
				found = true
				break
			}
		}
		if !found {
			result = append(result, a2)
		}
	}

	return result
}
