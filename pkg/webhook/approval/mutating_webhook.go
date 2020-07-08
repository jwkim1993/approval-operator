package approval

import (
	tmaxv1 "approval-operator/pkg/apis/tmax/v1"
	"context"
	"encoding/json"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Mutator struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (m *Mutator) Handle(_ context.Context, req admission.Request) admission.Response {
	reqLogger := logf.Log.WithName("webhook-approval-validating")

	// Requested content
	approval := &tmaxv1.Approval{}
	if err := m.decoder.Decode(req, approval); err != nil {
		reqLogger.Error(err, "unable to decode webhook request (object)")
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1beta1.Create:
		// At creation, add default values on spec, status
		if approval.Spec.AccessPath == "" {
			approval.Spec.AccessPath = "/"
		}
		if approval.Spec.Port == 0 {
			approval.Spec.Port = 10203
		}
		if approval.Spec.Threshold == 0 {
			approval.Spec.Threshold = 1
		}

		// Default to waiting status
		if len(approval.Status.Conditions) == 0 {
			waitCond := tmaxv1.Condition{
				Type:               tmaxv1.ConditionWaiting,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			}
			approval.Status.Conditions = append(approval.Status.Conditions, waitCond)
		}
	case admissionv1beta1.Update:
		// At update, just consider status.approvers field (for defaulting time of approval)
		for i, a := range approval.Status.Approvers {
			if a.ApprovedTime.IsZero() {
				approval.Status.Approvers[i].ApprovedTime = metav1.Now()
			}
		}

	}

	marshaledObj, err := json.Marshal(approval)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledObj)
}

func (m *Mutator) InjectClient(c client.Client) error {
	m.Client = c
	return nil
}

func (m *Mutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
