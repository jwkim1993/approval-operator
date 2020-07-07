package main

import (
	"approval-operator/internal"
	"approval-operator/pkg/apis"
	tmaxv1 "approval-operator/pkg/apis/tmax/v1"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"
	"time"
)

const (
	ApprovedMessage string = "Approval accepted. Exit the server."
	RejectedMessage string = "Reject accepted. Exit the server."
	UnknownMessage  string = "Decision Unknown: "

	OperatorSvcAddr string = "approval-operator.hypercloud4-system.svc.cluster.local/approval/"

	ConfigMapPath    string = "/tmp/config/users"
	AccessPath       string = "/"
	Port             int32  = 10203
	DefaultThreshold int    = 1
)

func messageHandler(w http.ResponseWriter, r *http.Request) {
	var m apis.ApprovedMessage
	err := json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		panic(err.Error())
	}

	exitCode := 0
	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")

	var msg string
	if m.Decision == tmaxv1.DecisionApproved {
		msg = ApprovedMessage
	} else if m.Decision == tmaxv1.DecisionRejected {
		msg = RejectedMessage
		exitCode = 1
	} else {
		log.Log.Info("Message: " + UnknownMessage)
		resMsg := apis.ApprovedMessage{Decision: tmaxv1.DecisionUnknown, Response: UnknownMessage + string(m.Decision)}
		err = enc.Encode(resMsg)
		if err != nil {
			panic(err.Error())
		}
		return
	}

	// approved or rejected
	log.Log.Info("Message: " + msg)
	resMsg := apis.ApprovedMessage{Decision: m.Decision, Response: msg}
	err = enc.Encode(resMsg)
	if err != nil {
		panic(err.Error())
	}

	// exit the server
	go func() {
		time.Sleep(5 * time.Second)
		os.Exit(exitCode)
	}()
}

func Users() (map[string]string, error) {
	file, err := os.Open(ConfigMapPath)
	if err != nil {
		return nil, errors.New("could not open config map")
	}
	defer file.Close()

	var users map[string]string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		user := strings.Split(scanner.Text(), "=")
		users[user[0]] = user[1]
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.New("error occurs on scanner")
	}

	return users, nil
}

func CreateApproval() error {
	namespace, err := internal.Namespace()
	if err != nil {
		return err
	}

	hostName, err := os.Hostname()
	if err != nil {
		return err
	}

	podIP, err := internal.LocalIP()
	if err != nil {
		return err
	}

	var threshold int
	thresEnv := os.Getenv("THRESHOLD")
	if thresEnv == "" {
		threshold = DefaultThreshold
	} else {
		threshold, err = strconv.Atoi(thresEnv)
		if err != nil {
			return errors.New("wrong threshold: " + thresEnv)
		}
	}

	users, err := Users()
	if err != nil {
		return err
	}

	msg := apis.PostApprovalMessage{
		Namespace:  namespace,
		PodName:    hostName,
		PodIP:      podIP,
		AccessPath: AccessPath,
		Port:       Port,
		Threshold:  int32(threshold),
		Users:      users,
	}

	msgByte, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	buff := bytes.NewBuffer(msgByte)
	_, err = http.Post(OperatorSvcAddr, "application/json", buff)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	// create Approval
	err := CreateApproval()
	if err != nil {
		panic(err.Error())
	}

	router := mux.NewRouter()
	router.HandleFunc("/", messageHandler).Methods("PUT")

	http.Handle("/", router)
	err = http.ListenAndServe(fmt.Sprintf(":%d", Port), nil)
	if err != nil {
		panic(err.Error())
	}
}
