package approval

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"approval-operator/internal"
	tmaxv1 "approval-operator/pkg/apis/tmax/v1"
)

type Validator struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (v *Validator) Handle(_ context.Context, req admission.Request) admission.Response {
	reqLogger := logf.Log.WithName("webhook-approval-validating")

	// Requested content
	approval := &tmaxv1.Approval{}
	if err := v.decoder.Decode(req, approval); err != nil {
		reqLogger.Error(err, "unable to decode webhook request (object)")
		return admission.Errored(http.StatusBadRequest, err)
	}

	reqLogger.Info(fmt.Sprintf("USER: %+v", req.UserInfo))

	// Validate contents at create
	if err := validate(approval); err != nil {
		reqLogger.Info(fmt.Sprintf("spec validation failed, err: %s", err.Error()))
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Authenticate at update
	if req.Operation == admissionv1beta1.Update {
		// If update is for spec, not for status, reject (spec is immutable after creation)
		if req.SubResource != "status" {
			errMsg := "updating spec field after creation is forbidden"
			err := errors.New(errMsg)
			reqLogger.Info(errMsg)
			return admission.Errored(http.StatusBadRequest, err)
		}

		// Old Approval
		oldApproval := &tmaxv1.Approval{}
		if err := v.decoder.DecodeRaw(req.OldObject, oldApproval); err != nil {
			reqLogger.Error(err, "unable to decode webhook request (oldObject)")
			return admission.Errored(http.StatusBadRequest, err)
		}

		// If update performed after approved/rejected, reject (all fields are immutable after final decision is made)
		approvedCond := oldApproval.Status.GetCondition(string(tmaxv1.DecisionApproved))
		rejectedCond := oldApproval.Status.GetCondition(string(tmaxv1.DecisionRejected))
		if (approvedCond != nil && approvedCond.Status == corev1.ConditionTrue) ||
			(rejectedCond != nil && rejectedCond.Status == corev1.ConditionTrue) {
			errMsg := "updating after rejected/approved is forbidden"
			err := errors.New(errMsg)
			reqLogger.Info(errMsg)
			return admission.Errored(http.StatusBadRequest, err)
		}

		// Authenticate at status change
		if err := authenticate(approval, oldApproval, req.UserInfo); err != nil {
			reqLogger.Info(fmt.Sprintf("authorization failed, err: %s", err.Error()))
			return admission.Errored(http.StatusUnauthorized, err)
		}
	}

	return admission.Allowed("")
}

func (v *Validator) InjectClient(c client.Client) error {
	v.Client = c
	return nil
}

func (v *Validator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

// Validate fields' values
func validate(approval *tmaxv1.Approval) error {
	// Port number validation
	if approval.Spec.Port < 1 || approval.Spec.Port > 65535 {
		return fmt.Errorf("port number(%d) is not in range of 1-65535", approval.Spec.Port)
	}

	// Path should start with /
	if approval.Spec.AccessPath == "" || approval.Spec.AccessPath[0] != '/' {
		return fmt.Errorf("access path(%s) does not start with slash(/)", approval.Spec.AccessPath)
	}

	// Pod IP validation
	if ip := net.ParseIP(approval.Spec.PodIP); ip == nil {
		return fmt.Errorf("podIP(%s) is not valid IP", approval.Spec.PodIP)
	}

	// Number of users should be greater than 0
	if len(approval.Spec.Users) < 1 {
		return fmt.Errorf("there should be one or more users specified")
	}

	// Threshold should be greater or equal to 1, less or equal to len(users)
	if approval.Spec.Threshold < 1 || int(approval.Spec.Threshold) > len(approval.Spec.Users) {
		return fmt.Errorf("threshold(%d) should be greater or equal to 1, less or equal to the length of users", approval.Spec.Threshold)
	}

	// Validate status field
	for i := range approval.Status.Approvers {
		for j := i + 1; j < len(approval.Status.Approvers); j++ {
			if approval.Status.Approvers[i].UserID == approval.Status.Approvers[j].UserID {
				return fmt.Errorf("duplicated user id(%s) in 'status.approvers' field", approval.Status.Approvers[i].UserID)
			}
		}
	}

	return nil
}

// Authenticate if the user requested the change is permitted to change specific field
func authenticate(approval *tmaxv1.Approval, oldApproval *tmaxv1.Approval, userInfo authenticationv1.UserInfo) error {
	status := approval.Status
	oldStatus := oldApproval.Status

	isOperator, err := isUserOperator(userInfo)
	if err != nil {
		return err
	}

	// Changes to status field is permitted only for operator and the users specified in spec.users
	_, exist := approval.Spec.Users[userInfo.Username]
	if !isOperator && !exist {
		return fmt.Errorf("user(%s) is not requested for the approval", userInfo.Username)
	}

	// Changed 'conditions' field --> permit only if user is operator
	if !reflect.DeepEqual(status.Conditions, oldStatus.Conditions) && !isOperator {
		return fmt.Errorf("only operator can update 'conditions' filed")
	}

	// Changed 'retry' field --> permit only if user is operator
	if status.Retry != oldStatus.Retry && !isOperator {
		return fmt.Errorf("only operator can update 'retry' filed")
	}

	// Changed 'approvers field' --> permit only if the user modified his/her field (if is operator, just permit)
	if !reflect.DeepEqual(status.Approvers, oldStatus.Approvers) && !isOperator {
		// Find updated approver
		changed := difference(status.Approvers, oldStatus.Approvers)
		for _, a := range changed {
			if a.UserID != userInfo.Username {
				return fmt.Errorf("changing other user's(%s) status field by a user(%s) is forbidden", a.UserID, userInfo.Username)
			}
		}
	}

	return nil
}

func isUserOperator(userInfo authenticationv1.UserInfo) (bool, error) {
	ns, err := internal.Namespace()
	if err != nil {
		return false, err
	}
	return userInfo.Username == fmt.Sprintf("system:serviceaccount:%s:approval-operator", ns), nil
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
