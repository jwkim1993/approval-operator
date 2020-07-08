package internal

import (
	"errors"
	"io/ioutil"
	"net"
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

// GetLocalIP returns the non loopback local IP of the host
func LocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", err
}

func WebhookServiceName() string {
	svcName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if svcName == "" {
		svcName = "approval-operator"
	}
	return svcName
}
