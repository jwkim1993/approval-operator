package internal

import (
	"errors"
	"io/ioutil"
	"os"
)

func Namespace() (string, error) {
	nsPath := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	if FileExists(nsPath) {
		// Running in k8s cluster
		nsBytes, err := ioutil.ReadFile(nsPath)
		if err != nil {
			return "", errors.New("could not read file " + nsPath)
		}
		return string(nsBytes), nil
	} else {
		// Not running in k8s cluster (may be running locally)
		ns := os.Getenv("NAMESPACE")
		if ns == "" {
			ns = "default"
		}
		return ns, nil
	}
}

func WebhookServiceName() string {
	svcName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if svcName == "" {
		svcName = "approval-webhook"
	}
	return svcName
}
