package approval

import (
	"os"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	DefaultPort          = 443
	CertDir              = "/tmp/approval-webhook"
	ValidationPath       = "/validate-approvals"
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
