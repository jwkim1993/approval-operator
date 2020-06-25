package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"errors"
	"time"
)

type ApprovalStatus struct {
	Conditions	Conditions	`json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	Approvers	[]Approver	`json:"approvers,omitempty"`
	Retry		int32		`json:"retry"`
}

type Approver struct {
	UserID		string		`json:"userId"`
	ApprovedTime	metav1.Time	`json:"approvedTime"`
}

func (s *ApprovalStatus) GetCondition(t string) *Condition {
	for _, cond := range s.Conditions {
		if cond.Type == t {
			return &cond
		}
	}

	return nil
}

func (s *ApprovalStatus) SetApprover(u string) error {
	for _, appr := range s.Approvers {
		if appr.UserID == u {
			return errors.New("duplicated user id")
		}
	}

	s.Approvers = append(s.Approvers, Approver{
							UserID: u,
							ApprovedTime: metav1.NewTime(time.Now()),
						})

	return nil
}

func (s *ApprovalStatus) IsApproversOverThreshold(thres int) bool {
	if len(s.Approvers) >= thres {
		return true
	}
	return false
}