package approval

import (
	"approval-operator/pkg/apis"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"time"

	tmaxv1 "approval-operator/pkg/apis/tmax/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_approval")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Approval Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileApproval{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("approval-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Approval
	err = c.Watch(&source.Kind{Type: &tmaxv1.Approval{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileApproval implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileApproval{}

// ReconcileApproval reconciles a Approval object
type ReconcileApproval struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Approval object and makes changes based on the state read
// and what is in the Approval.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileApproval) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Approval")

	// Fetch the Approval instance
	instance := &tmaxv1.Approval{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Approval:", instance.Name, "in namespace:", instance.Namespace, "is deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Approval")
		return reconcile.Result{}, err
	}

	if len(instance.Status.Conditions) == 0 {
		reqLogger.Info("Approval initialize. Set Waiting status.")
		if err = r.setStatus(instance, tmaxv1.ConditionWaiting); err != nil {
			reqLogger.Error(err, "Failed to set Waiting status")
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// If condition is not "Waiting" and Status true, end this function
	for _, cond := range instance.Status.Conditions {
		if cond.Type != tmaxv1.ConditionWaiting && cond.Status == corev1.ConditionTrue {
			reqLogger.Info("The approving process already ended")
			return reconcile.Result{}, nil
		}
	}

	// If there are any rejection, send reject message
	for _, appr := range instance.Status.Approvers {
		if appr.Decision == tmaxv1.DecisionRejected {
			if err := r.sendMsgToTask(instance, tmaxv1.DecisionRejected); err != nil {
				reqLogger.Error(err, "Failed to send reject msg to Task")
				// instance.Status.Condition create failed condition and reason
				return reconcile.Result{}, err
			}

			// if succeed to send msg, change status
			if err := r.setStatus(instance, tmaxv1.ConditionRejected); err != nil {
				reqLogger.Error(err, "Failed to set status")
				return reconcile.Result{}, err
			}

			// rejection success
			return reconcile.Result{}, nil
		}
	}

	// If any of approvals make rejection and the number of approvals is over the threshold,
	if instance.Status.IsApproversOverThreshold(int(instance.Spec.Threshold)) {
		if err := r.sendMsgToTask(instance, tmaxv1.DecisionApproved); err != nil {
			reqLogger.Error(err, "Failed to send approve msg to Task")
			//instance.Status.Conditions create failed condition and reason
			return reconcile.Result{}, err
		}

		// if succeed to send msg, change status
		if err := r.setStatus(instance, tmaxv1.ConditionApproved); err != nil {
			reqLogger.Error(err, "Failed to set status")
			return reconcile.Result{}, err
		}

		// approved success
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileApproval) setStatus(cr *tmaxv1.Approval, ct tmaxv1.ConditionType) error {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.Name)
	newCondition := tmaxv1.Condition{
		Type:               ct,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             "",
		Message:            "",
	}

	if len(cr.Status.Conditions) != 0 {
		cr.Status.Conditions = cr.Status.Conditions[1:]
	}
	cr.Status.Conditions = append(cr.Status.Conditions, newCondition)

	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		reqLogger.Error(err, "Unknown error updating status")
		return err
	}

	return nil
}

func (r *ReconcileApproval) sendMsgToTask(cr *tmaxv1.Approval, dt tmaxv1.DecisionType) error {

	data := apis.ApprovedMessage{
		Decision: dt,
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("PUT", fmt.Sprint(cr.Spec.PodIP, ":", cr.Spec.Port, "/", cr.Spec.AccessPath), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
