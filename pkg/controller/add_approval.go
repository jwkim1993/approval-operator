package controller

import (
	"approval-operator/internal"
	"approval-operator/pkg/apis"
	tmaxv1 "approval-operator/pkg/apis/tmax/v1"
	"approval-operator/pkg/controller/approval"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/prometheus/common/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Port int = 8081
)

func approvalCreator(w http.ResponseWriter, r *http.Request) {
	var m apis.PostApprovalMessage
	err := json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		log.Error(err, "Cannot decode the message")
		return
	}

	c, err := internal.Client(client.Options{})
	if err != nil {
		log.Error(err, "Cannot create simple client")
		return
	}

	labels := make(map[string]string)
	for k := range m.Users {
		labels[k] = ""
	}

	newApproval := &tmaxv1.Approval{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", m.PodName),
			Namespace:    m.Namespace,
			Labels:       labels,
		},
		Spec: tmaxv1.ApprovalSpec{
			PodIP:      m.PodIP,
			AccessPath: m.AccessPath,
			Port:       m.Port,
			Threshold:  m.Threshold,
			Users:      m.Users,
		},
	}

	err = c.Create(context.TODO(), newApproval)
	if err != nil {
		log.Error("Cannot create approval: " + err.Error())
		return
	}

	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	err = enc.Encode(m)
	if err != nil {
		log.Error("Cannot reply request: " + err.Error())
		return
	}
}

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, approval.Add)

	go func() {
		router := mux.NewRouter()
		router.HandleFunc("/approval", approvalCreator).Methods("POST")

		http.Handle("/", router)
		err := http.ListenAndServe(fmt.Sprintf(":%d", Port), nil)
		if err != nil {
			panic(err.Error())
		}
	}()
}
