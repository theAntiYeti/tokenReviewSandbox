package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
)

var (
	addr = flag.String("addr", "localhost:50051", "the address to connect to")
)

func main() {
	flag.Parse()

	token, err := createAuthenticationToken()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	conn, err := grpc.Dial(*addr, grpc.WithPerRPCCredentials(token), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "a dummy name"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}

	log.Printf("Greeting: %s", r)
	// Sleep long time
	time.Sleep(1000000 * time.Hour)
}

func createAuthenticationToken() (*TokenCredentials, error) {
	// Get the main service account token for getting a tmp token, do not send this to server
	log.Info("Getting service account token")
	kubernetesToken, err := getKubernetesToken()
	if err != nil {
		log.Infof("Error getting service account token: %v", err)
	}
	log.Infof("Service account token: %v", kubernetesToken)

	var bearer = "Bearer " + kubernetesToken
	var myUrl = "https://kubernetes.default.svc/api/v1/namespaces/default/serviceaccounts/admin-user/token"

	log.Infof("Attempting to call %s", myUrl)
	req, err := http.NewRequest("POST", myUrl, bytes.NewBuffer([]byte("{}")))
	req.Header.Add("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		log.Infof("Error getting temp token: %v", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	token, err := parseTokenString(body)

	log.Infof("Token: %s", token)

	return &TokenCredentials{token: token}, err
}

func getKubernetesToken() (string, error) {
	fromEnv := os.Getenv("K8S_SERVICEACCOUNT_TOKEN")
	if fromEnv != "" {
		return fromEnv, nil
	}

	fromFile, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", err
	}

	return string(fromFile), nil
}

func parseTokenString(body []byte) (string, error) {
	var statusRes unmarshalAll
	if err := json.Unmarshal(body, &statusRes); err != nil {
		return "", fmt.Errorf("could not deserialise token: %v", err)
	}
	log.Infof("Status is: %v", statusRes.Status)

	return statusRes.Status.Token, nil
}

type unmarshalStatus struct {
	Token string `json:"token"`
}

type unmarshalAll struct {
	Status unmarshalStatus `json:"status"`
}

type TokenCredentials struct {
	token string
}

func (c *TokenCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + c.token,
	}, nil
}

func (c *TokenCredentials) RequireTransportSecurity() bool {
	return false
}
