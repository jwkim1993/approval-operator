package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

type Conditions []Condition

// to seperate conditions and our status. conditions will be replaced by knative.conditions
type Condition struct {
        // Type of condition.
        // +required
        Type string `json:"type" description:"type of status condition"`

        // Status of the condition, one of True, False, Unknown.
        // +required
        Status corev1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`

        // LastTransitionTime is the last time the condition transitioned from one status to another.
        // We use VolatileTime in place of metav1.Time to exclude this from creating equality.Semantic
        // differences (all other things held constant).
        // +optional
        LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" description:"last time the condition transit from one status to another"`

        // The reason for the condition's last transition.
        // +optional
        Reason string `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`

        // A human readable message indicating details about the transition.
        // +optional
        Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// IsTrue is true if the condition is True
func (c *Condition) IsTrue() bool {
	if c == nil {
		return false
	}
	return c.Status == corev1.ConditionTrue
}

// IsFalse is true if the condition is False
func (c *Condition) IsFalse() bool {
	if c == nil {
		return false
	}
	return c.Status == corev1.ConditionFalse
}

// IsUnknown is true if the condition is Unknown
func (c *Condition) IsUnknown() bool {
	if c == nil {
		return true
	}
	return c.Status == corev1.ConditionUnknown
}

// GetReason returns a nil save string of Reason
func (c *Condition) GetReason() string {
	if c == nil {
		return ""
	}
	return c.Reason
}

// GetMessage returns a nil save string of Message
func (c *Condition) GetMessage() string {
	if c == nil {
		return ""
	}
	return c.Message
}
