package apis

import tmaxv1 "approval-operator/pkg/apis/tmax/v1"

type ApprovedMessage struct {
	Decision tmaxv1.DecisionType `json:"decision"`
	Response string              `json:"response"`
}

type PostApprovalMessage struct {
	Namespace  string            `json:"namespace"`
	PodName    string            `json:"podName"`
	PodIP      string            `json:"podIP"`
	Threshold  int32             `json:"threshold"`
	AccessPath string            `json:"accessPath"`
	Port       int32             `json:"port"`
	Users      map[string]string `json:"users"`
}
