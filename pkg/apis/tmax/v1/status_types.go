package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"time"
)

// DecisionType field should have Approved or Rejected.
// +kubebuilder:validation:Enum=Approved;Rejected
type DecisionType string

const (
	DecisionApproved DecisionType = "Approved"
	DecisionRejected DecisionType = "Rejected"
)

type ApprovalStatus struct {
	Conditions Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	Approvers  []Approver `json:"approvers,omitempty"`
	// +optional
	// +kubebuilder:default:=0
	Retry int32 `json:"retry"`
}

type Approver struct {
	UserID       string       `json:"userId"`
	Decision     DecisionType `json:"decision"`
	ApprovedTime metav1.Time  `json:"approvedTime"`
}

func (s *ApprovalStatus) GetCondition(t string) *Condition {
	for _, cond := range s.Conditions {
		if cond.Type == t {
			return &cond
		}
	}

	return nil
}

func (s *ApprovalStatus) GetApprover(u string) *Approver {
	for i := range s.Approvers {
		if s.Approvers[i].UserID == u {
			return &s.Approvers[i]
		}
	}
	return nil
}

func (s *ApprovalStatus) SetApprover(u string, d DecisionType) {
	for i := range s.Approvers {
		if s.Approvers[i].UserID == u {
			s.Approvers[i].Decision = d
			s.Approvers[i].ApprovedTime = metav1.NewTime(time.Now())
			return
		}
	}

	s.Approvers = append(s.Approvers, Approver{
		UserID:       u,
		Decision:     d,
		ApprovedTime: metav1.NewTime(time.Now()),
	})

}

func (s *ApprovalStatus) IsApproversOverThreshold(thres int) bool {
	return len(s.Approvers) >= thres
}
